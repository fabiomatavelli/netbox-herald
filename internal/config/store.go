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

// Package config holds the HeraldConfig spec and authenticated NetBox
// client shared between the HeraldConfig reconciler and every resource-type
// reconciler, so only one reconciler ever talks to the Kubernetes API server
// to resolve HeraldConfig and its referenced Secret.
package config

import (
	"sync"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"

	heraldv1alpha1 "github.com/fabiomatavelli/netbox-herald/api/v1alpha1"
)

// Store holds the most recently reconciled HeraldConfig spec and its
// authenticated NetBox client. It is safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	spec   *heraldv1alpha1.HeraldConfigSpec
	client *netboxclient.NetBoxAPI
}

// NewStore returns an empty Store. Get returns nil, nil until Set has been
// called at least once.
func NewStore() *Store {
	return &Store{}
}

// Set replaces the stored spec and client. Called only by the HeraldConfig
// reconciler, after it successfully builds a NetBox client for the current
// spec.
func (s *Store) Set(spec *heraldv1alpha1.HeraldConfigSpec, client *netboxclient.NetBoxAPI) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spec = spec
	s.client = client
}

// Get returns the current spec and client. Both are nil until Set has been
// called successfully at least once.
func (s *Store) Get() (*heraldv1alpha1.HeraldConfigSpec, *netboxclient.NetBoxAPI) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.spec, s.client
}
