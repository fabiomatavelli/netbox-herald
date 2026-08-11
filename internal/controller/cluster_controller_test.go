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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
	"github.com/fabiomatavelli/netbox-herald/internal/config"
)

var _ = Describe("Cluster Controller", func() {
	var reconciler *ClusterReconciler

	BeforeEach(func() {
		reconciler = &ClusterReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
			Store:  config.NewStore(),
		}
	})

	It("ignores a HeraldConfig not named \"default\"", func() {
		ctx := context.Background()

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: "not-the-singleton"},
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("no-ops until HeraldConfigReconciler has populated the Store", func() {
		ctx := context.Background()

		cr := &heraldv1alpha1.HeraldConfig{
			ObjectMeta: metav1.ObjectMeta{Name: singletonName},
			Spec: heraldv1alpha1.HeraldConfigSpec{
				NetBox: heraldv1alpha1.NetBoxConfig{
					URL:            "http://127.0.0.1:0",
					TokenSecretRef: heraldv1alpha1.SecretKeyRef{Name: "does-not-matter"},
				},
				Resources: heraldv1alpha1.ResourcesConfig{
					Cluster: heraldv1alpha1.ClusterResourceConfig{Enabled: true},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cr)).To(Succeed()) })

		// The Store is empty (as it is until HeraldConfigReconciler
		// successfully connects), so Reconcile must not attempt to sync
		// anything or fail — it should quietly wait for the Store to be
		// populated.
		result, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: cr.Name},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsZero()).To(BeTrue())

		var updated heraldv1alpha1.HeraldConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cr.Name}, &updated)).To(Succeed())
		Expect(updated.Status.Resources.Cluster).To(Equal(heraldv1alpha1.ResourceSyncStatus{}))
	})
})
