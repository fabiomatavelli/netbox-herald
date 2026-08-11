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

// Package netbox wraps github.com/fbreckle/go-netbox with the
// authentication, TLS, and idempotency conventions netbox-herald needs.
package netbox

import (
	"crypto/x509"
	"fmt"
	"net/http"
	"strings"
	"time"

	netboxclient "github.com/fbreckle/go-netbox/netbox/client"
	"github.com/go-logr/logr"
	httptransport "github.com/go-openapi/runtime/client"
	"github.com/goware/urlx"
)

// Config describes how to connect to a NetBox instance.
type Config struct {
	// ServerURL is the base URL of the NetBox instance, e.g.
	// https://netbox.example.com.
	ServerURL string

	// APIToken authenticates requests. A "nbt_" prefix selects NetBox's v2
	// Bearer token scheme; anything else uses the legacy Token scheme.
	APIToken string

	// InsecureSkipVerify disables TLS certificate verification.
	InsecureSkipVerify bool

	// CACertPEM is an optional PEM-encoded CA bundle to trust in addition to
	// the system roots.
	CACertPEM []byte

	// RequestTimeout bounds how long a single request may take.
	RequestTimeout time.Duration

	// Logger receives request/response logging from the underlying
	// go-openapi runtime transport.
	Logger logr.Logger
}

// Client builds an authenticated NetBox API client from the Config.
func (cfg Config) Client() (*netboxclient.NetBoxAPI, error) {
	if cfg.APIToken == "" {
		return nil, fmt.Errorf("missing NetBox API token")
	}

	parsedURL, err := urlx.Parse(cfg.ServerURL)
	if err != nil {
		return nil, fmt.Errorf("parsing NetBox server URL: %w", err)
	}

	tlsOpts := httptransport.TLSClientOptions{
		InsecureSkipVerify: cfg.InsecureSkipVerify,
	}
	if len(cfg.CACertPEM) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(cfg.CACertPEM) {
			return nil, fmt.Errorf("no valid certificates found in CA bundle")
		}
		tlsOpts.LoadedCAPool = pool
	}

	trans, err := httptransport.TLSTransport(tlsOpts)
	if err != nil {
		return nil, fmt.Errorf("building TLS transport: %w", err)
	}
	trans.(*http.Transport).Proxy = http.ProxyFromEnvironment

	requestTimeout := cfg.RequestTimeout
	if requestTimeout <= 0 {
		requestTimeout = 30 * time.Second
	}

	httpClient := &http.Client{
		Transport: trans,
		Timeout:   requestTimeout,
	}

	schemes := []string{parsedURL.Scheme}
	runtimeTransport := httptransport.NewWithClient(parsedURL.Host, parsedURL.Path+netboxclient.DefaultBasePath, schemes, httpClient)

	authScheme := "Token"
	if strings.HasPrefix(cfg.APIToken, "nbt_") {
		authScheme = "Bearer"
	}
	runtimeTransport.DefaultAuthentication = httptransport.APIKeyAuth("Authorization", "header", fmt.Sprintf("%s %s", authScheme, cfg.APIToken))
	runtimeTransport.SetLogger(runtimeLogger{cfg.Logger})

	return netboxclient.New(runtimeTransport, nil), nil
}

// runtimeLogger adapts a logr.Logger to the go-openapi runtime's Debugf/
// Printf-style Logger interface.
type runtimeLogger struct {
	log logr.Logger
}

func (l runtimeLogger) Printf(format string, args ...any) {
	l.log.Info(fmt.Sprintf(format, args...))
}

func (l runtimeLogger) Debugf(format string, args ...any) {
	l.log.V(1).Info(fmt.Sprintf(format, args...))
}
