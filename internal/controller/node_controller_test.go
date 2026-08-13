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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
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
			NamespacedName: types.NamespacedName{Name: doesNotExistName},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(storeNotReadyPollInterval))
	})
})

var _ = Describe("nodePrimaryAddresses", func() {
	const testIPv4 = "10.13.11.50"

	nodeWithAddresses := func(addrs ...corev1.NodeAddress) *corev1.Node {
		return &corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "test-node"},
			Status:     corev1.NodeStatus{Addresses: addrs},
		}
	}

	It("returns the InternalIP when only an IPv4 address is present", func() {
		node := nodeWithAddresses(corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: testIPv4})
		ipv4, ipv6, err := nodePrimaryAddresses(node, heraldv1alpha1.NodeAddressTypeInternalIP)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipv4).To(Equal(testIPv4 + "/32"))
		Expect(ipv6).To(BeEmpty())
	})

	It("returns both addresses for a dual-stack node", func() {
		node := nodeWithAddresses(
			corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: testIPv4},
			corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "2001:db8::1"},
		)
		ipv4, ipv6, err := nodePrimaryAddresses(node, heraldv1alpha1.NodeAddressTypeInternalIP)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipv4).To(Equal(testIPv4 + "/32"))
		Expect(ipv6).To(Equal("2001:db8::1/128"))
	})

	It("only considers addresses of the configured type", func() {
		node := nodeWithAddresses(
			corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: testIPv4},
			corev1.NodeAddress{Type: corev1.NodeExternalIP, Address: "203.0.113.5"},
		)
		ipv4, ipv6, err := nodePrimaryAddresses(node, heraldv1alpha1.NodeAddressTypeExternalIP)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipv4).To(Equal("203.0.113.5/32"))
		Expect(ipv6).To(BeEmpty())
	})

	It("returns empty strings and no error when no address of the configured type exists", func() {
		node := nodeWithAddresses(corev1.NodeAddress{Type: corev1.NodeHostName, Address: "test-node"})
		ipv4, ipv6, err := nodePrimaryAddresses(node, heraldv1alpha1.NodeAddressTypeInternalIP)
		Expect(err).NotTo(HaveOccurred())
		Expect(ipv4).To(BeEmpty())
		Expect(ipv6).To(BeEmpty())
	})

	It("errors on a malformed address value", func() {
		node := nodeWithAddresses(corev1.NodeAddress{Type: corev1.NodeInternalIP, Address: "not-an-ip"})
		_, _, err := nodePrimaryAddresses(node, heraldv1alpha1.NodeAddressTypeInternalIP)
		Expect(err).To(HaveOccurred())
	})
})
