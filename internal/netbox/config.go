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

package netbox

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
)

// LoadConfig resolves a HeraldConfig's spec.netbox block into a Config,
// reading the referenced token and (optional) CA bundle Secret keys via
// k8sClient. secretNamespace is the namespace the referenced Secrets are
// read from — the operator's own namespace, since HeraldConfig itself is
// cluster-scoped.
func LoadConfig(ctx context.Context, k8sClient client.Client, secretNamespace string, spec heraldv1alpha1.NetBoxConfig) (Config, error) {
	token, err := secretKey(ctx, k8sClient, secretNamespace, spec.TokenSecretRef)
	if err != nil {
		return Config{}, fmt.Errorf("resolving netbox.tokenSecretRef: %w", err)
	}

	cfg := Config{
		ServerURL:          spec.URL,
		APIToken:           string(token),
		InsecureSkipVerify: spec.TLS.InsecureSkipVerify,
		RequestTimeout:     time.Duration(spec.RequestTimeoutSeconds) * time.Second,
		Logger:             logr.FromContextOrDiscard(ctx),
	}

	if spec.TLS.CABundleSecretRef != nil {
		caPEM, err := secretKey(ctx, k8sClient, secretNamespace, *spec.TLS.CABundleSecretRef)
		if err != nil {
			return Config{}, fmt.Errorf("resolving netbox.tls.caBundleSecretRef: %w", err)
		}
		cfg.CACertPEM = caPEM
	}

	return cfg, nil
}

func secretKey(ctx context.Context, k8sClient client.Client, namespace string, ref heraldv1alpha1.SecretKeyRef) ([]byte, error) {
	key := ref.Key
	if key == "" {
		key = "token"
	}

	var secret corev1.Secret
	if err := k8sClient.Get(ctx, types.NamespacedName{Namespace: namespace, Name: ref.Name}, &secret); err != nil {
		return nil, fmt.Errorf("getting secret %s/%s: %w", namespace, ref.Name, err)
	}

	value, ok := secret.Data[key]
	if !ok {
		return nil, fmt.Errorf("secret %s/%s has no key %q", namespace, ref.Name, key)
	}

	return value, nil
}
