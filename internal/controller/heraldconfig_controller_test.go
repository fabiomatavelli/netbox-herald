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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
	"github.com/fabiomatavelli/netbox-herald/internal/config"
)

var _ = Describe("HeraldConfig Controller", func() {
	const operatorNamespace = "default"

	var reconciler *HeraldConfigReconciler

	BeforeEach(func() {
		reconciler = &HeraldConfigReconciler{
			Client:            k8sClient,
			Scheme:            k8sClient.Scheme(),
			OperatorNamespace: operatorNamespace,
			Store:             config.NewStore(),
		}
	})

	It("ignores a HeraldConfig not named \"default\"", func() {
		ctx := context.Background()

		cr := &heraldv1alpha1.HeraldConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "not-the-singleton"},
			Spec: heraldv1alpha1.HeraldConfigSpec{
				NetBox: heraldv1alpha1.NetBoxConfig{
					URL:            "http://127.0.0.1:0",
					TokenSecretRef: heraldv1alpha1.SecretKeyRef{Name: "does-not-matter"},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cr)).To(Succeed()) })

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: cr.Name},
		})
		Expect(err).NotTo(HaveOccurred())

		spec, client := reconciler.Store.Get()
		Expect(spec).To(BeNil())
		Expect(client).To(BeNil())
	})

	It("reports NetBoxReachable=False when NetBox can't be reached", func() {
		ctx := context.Background()

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "netbox-token", Namespace: operatorNamespace},
			StringData: map[string]string{"token": "fake-token"},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, secret)).To(Succeed()) })

		cr := &heraldv1alpha1.HeraldConfig{
			ObjectMeta: metav1.ObjectMeta{Name: singletonName},
			Spec: heraldv1alpha1.HeraldConfigSpec{
				NetBox: heraldv1alpha1.NetBoxConfig{
					// Port 0 never accepts connections, so this always fails fast.
					URL:            "http://127.0.0.1:0",
					TokenSecretRef: heraldv1alpha1.SecretKeyRef{Name: secret.Name},
				},
			},
		}
		Expect(k8sClient.Create(ctx, cr)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, cr)).To(Succeed()) })

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{Name: cr.Name},
		})
		Expect(err).To(HaveOccurred())

		var updated heraldv1alpha1.HeraldConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cr.Name}, &updated)).To(Succeed())
		Expect(updated.Status.NetBox.Connected).To(BeFalse())

		cond := meta.FindStatusCondition(updated.Status.Conditions, "NetBoxReachable")
		Expect(cond).NotTo(BeNil())
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))

		spec, client := reconciler.Store.Get()
		Expect(spec).To(BeNil())
		Expect(client).To(BeNil())
	})
})
