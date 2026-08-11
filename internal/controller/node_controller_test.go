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

var _ = Describe("Node Controller", func() {
	var reconciler *NodeReconciler

	BeforeEach(func() {
		reconciler = &NodeReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Store:  config.NewStore(),
		}
	})

	It("no-ops until HeraldConfigReconciler has populated the Store", func() {
		ctx := context.Background()

		// No Node named "does-not-exist" exists either, but the Store check
		// happens first, so this must return cleanly without ever trying to
		// reach NetBox or read the Node — just poll again shortly, since
		// nothing else wakes it once the Store is populated.
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "does-not-exist"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(storeNotReadyPollInterval))
	})
})
