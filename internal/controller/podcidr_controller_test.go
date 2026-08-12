/*
Copyright 2026 Fábio Matavelli.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/fabiomatavelli/netbox-herald/internal/config"
)

var _ = Describe("PodCIDR Controller", func() {
	var reconciler *PodCIDRReconciler

	BeforeEach(func() {
		reconciler = &PodCIDRReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Store:  config.NewStore(),
		}
	})

	It("no-ops until HeraldConfigReconciler has populated the Store", func() {
		ctx := context.Background()

		// No Node named doesNotExistName exists either, but the Store check
		// happens first, so this must return cleanly without ever trying to
		// reach NetBox or read the Node — just poll again shortly, since
		// nothing else wakes it once the Store is populated.
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: doesNotExistName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(storeNotReadyPollInterval))
	})
})

var _ = Describe("computeSupernets", func() {
	const podCIDRNode0 = "10.244.0.0/24"
	const podCIDRNode1 = "10.244.1.0/24"
	const podCIDRNode2 = "10.244.2.0/24"

	It("returns a single family's exact CIDR when only one node CIDR exists", func() {
		supernets, err := computeSupernets([]string{podCIDRNode1})
		Expect(err).NotTo(HaveOccurred())
		Expect(supernets).To(Equal(map[string]string{familyIPv4: podCIDRNode1}))
	})

	It("computes the smallest enclosing block across several node CIDRs", func() {
		supernets, err := computeSupernets([]string{podCIDRNode0, podCIDRNode1, podCIDRNode2})
		Expect(err).NotTo(HaveOccurred())
		Expect(supernets).To(Equal(map[string]string{familyIPv4: "10.244.0.0/22"}))
	})

	It("keeps address families separate for dual-stack clusters", func() {
		supernets, err := computeSupernets([]string{
			podCIDRNode0,
			podCIDRNode1,
			"fd00:10:244::/64",
			"fd00:10:244:1::/64",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(supernets).To(Equal(map[string]string{
			familyIPv4: "10.244.0.0/23",
			familyIPv6: "fd00:10:244::/63",
		}))
	})

	It("returns an empty result for no input", func() {
		supernets, err := computeSupernets(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(supernets).To(BeEmpty())
	})

	It("rejects an invalid CIDR", func() {
		_, err := computeSupernets([]string{"not-a-cidr"})
		Expect(err).To(HaveOccurred())
	})
})
