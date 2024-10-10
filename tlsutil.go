// Just this file:
// Copyright 2014 The Cockroach Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
// implied. See the License for the specific language governing
// permissions and limitations under the License.

package main

import (
	"crypto/tls"
	"crypto/x509"
	"io/ioutil"

	"github.com/pkg/errors"
)

/*
creation:
// 878400h is 100 years.
mkdir certs my-safe-directory
cockroach cert create-ca  --lifetime=878400h --certs-dir=certs --ca-key=my-safe-directory/ca.key
cockroach cert create-client root --lifetime=878400h --certs-dir=certs --ca-key=my-safe-directory/ca.key
cockroach cert create-node localhost $(hostname)  --lifetime=878400h --certs-dir=certs --ca-key=my-safe-directory/ca.key

ref: https://www.cockroachlabs.com/docs/stable/secure-a-cluster.html
*/

// EmbeddedCertsDir is the certs directory inside embedded assets.
// Embedded*{Cert,Key} are the filenames for embedded certs.
/*const (
	EmbeddedCertsDir     = "test_certs"
	EmbeddedCACert       = "ca.crt"
	EmbeddedCAKey        = "ca.key"
	EmbeddedClientCACert = "ca-client.crt"
	EmbeddedClientCAKey  = "ca-client.key"
	EmbeddedUICACert     = "ca-ui.crt"
	EmbeddedUICAKey      = "ca-ui.key"
	EmbeddedNodeCert     = "node.crt"
	EmbeddedNodeKey      = "node.key"
	EmbeddedRootCert     = "client.crt"
	EmbeddedRootKey      = "client.key"
	EmbeddedTestUserCert = "client.testuser.crt"
	EmbeddedTestUserKey  = "client.testuser.key"
)
*/

// LoadServerTLSConfig creates a server TLSConfig by loading the CA and server certs.
// The following paths must be passed:
//   - sslCA: path to the CA certificate
//   - sslClientCA: path to the CA certificate to verify client certificates,
//     can be the same as sslCA
//   - sslCert: path to the server certificate
//   - sslCertKey: path to the server key
func LoadServerTLSConfig(sslCA, sslClientCA, sslCert, sslCertKey string) (*tls.Config, error) {

	certPEM, err := ioutil.ReadFile(sslCert)
	if err != nil {
		return nil, err
	}
	keyPEM, err := ioutil.ReadFile(sslCertKey)
	if err != nil {
		return nil, err
	}
	caPEM, err := ioutil.ReadFile(sslCA)
	if err != nil {
		return nil, err
	}
	clientCAPEM, err := ioutil.ReadFile(sslClientCA)
	if err != nil {
		return nil, err
	}
	return newServerTLSConfig(certPEM, keyPEM, caPEM, clientCAPEM)
}

// newServerTLSConfig creates a server TLSConfig from the supplied byte strings containing
// - the certificate of this node (should be signed by the CA),
// - the private key of this node.
// - the certificate of the cluster CA, used to verify other server certificates
// - the certificate of the client CA, used to verify client certificates
//
// caClientPEM can be equal to caPEM (shared CA) or nil (use system CA pool).
func newServerTLSConfig(certPEM, keyPEM, caPEM, caClientPEM []byte) (*tls.Config, error) {
	cfg, err := newBaseTLSConfigWithCertificate(certPEM, keyPEM, caPEM)
	if err != nil {
		return nil, err
	}
	cfg.ClientAuth = tls.RequireAndVerifyClientCert // jea was: tls.VerifyClientCertIfGiven

	if caClientPEM != nil {
		certPool := x509.NewCertPool()

		if !certPool.AppendCertsFromPEM(caClientPEM) {
			return nil, errors.Errorf("failed to parse client CA PEM data to pool")
		}
		cfg.ClientCAs = certPool
	}

	// Use the default cipher suite from golang (RC4 is going away in 1.5).
	// Prefer the server-specified suite.
	cfg.PreferServerCipherSuites = true
	// Should we disable session resumption? This may break forward secrecy.
	// cfg.SessionTicketsDisabled = true
	return cfg, nil
}

// newUIServerTLSConfig creates a server TLSConfig for the Admin UI. It does not
// use client authentication or a CA.
// It needs:
// - the server certificate (should be signed by the CA used by HTTP clients to the admin UI)
// - the private key for the certificate
func newUIServerTLSConfig(certPEM, keyPEM []byte) (*tls.Config, error) {
	cfg, err := newBaseTLSConfigWithCertificate(certPEM, keyPEM, nil)
	if err != nil {
		return nil, err
	}

	// Use the default cipher suite from golang (RC4 is going away in 1.5).
	// Prefer the server-specified suite.
	// update: PreferServerCipherSuites is a legacy field and has no effect.
	// cfg.PreferServerCipherSuites = true
	//
	// Should we disable session resumption? This may break forward secrecy.
	// cfg.SessionTicketsDisabled = true
	return cfg, nil
}

// LoadClientTLSConfig creates a client TLSConfig by loading the CA and client certs.
// The following paths must be passed:
// - sslCA: path to the CA certificate
// - sslCert: path to the client certificate
// - sslCertKey: path to the client key
// If the path is prefixed with "embedded=", load the embedded certs.
//
// embedded true means load from the static embedded files, created
// at compile time. If embedded is false, then we read from the filesystem.
func LoadClientTLSConfigFilesystem(sslCA, sslCert, sslCertKey string) (*tls.Config, error) {
	certPEM, err := ioutil.ReadFile(sslCert)
	if err != nil {
		return nil, err
	}
	keyPEM, err := ioutil.ReadFile(sslCertKey)
	if err != nil {
		return nil, err
	}
	caPEM, err := ioutil.ReadFile(sslCA)
	if err != nil {
		return nil, err
	}

	return newClientTLSConfig(certPEM, keyPEM, caPEM)
}

// newClientTLSConfig creates a client TLSConfig from the supplied byte strings containing:
// - the certificate of this client (should be signed by the CA),
// - the private key of this client.
// - the certificate of the cluster CA (use system cert pool if nil)
func newClientTLSConfig(certPEM, keyPEM, caPEM []byte) (*tls.Config, error) {
	return newBaseTLSConfigWithCertificate(certPEM, keyPEM, caPEM)
}

// newUIClientTLSConfig creates a client TLSConfig to talk to the Admin UI.
// It does not include client certificates and takes an optional CA certificate.
func newUIClientTLSConfig(caPEM []byte) (*tls.Config, error) {
	return newBaseTLSConfig(caPEM)
}

// newBaseTLSConfigWithCertificate returns a tls.Config initialized with the
// passed-in certificate and optional CA certificate.
func newBaseTLSConfigWithCertificate(certPEM, keyPEM, caPEM []byte) (*tls.Config, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return nil, err
	}

	cfg, err := newBaseTLSConfig(caPEM)
	if err != nil {
		return nil, err
	}

	cfg.Certificates = []tls.Certificate{cert}
	return cfg, nil
}

// newBaseTLSConfig returns a tls.Config. If caPEM != nil, it is set in RootCAs.
func newBaseTLSConfig(caPEM []byte) (*tls.Config, error) {
	var certPool *x509.CertPool
	if caPEM != nil {
		certPool = x509.NewCertPool()

		if !certPool.AppendCertsFromPEM(caPEM) {
			return nil, errors.Errorf("failed to parse PEM data to pool")
		}
	}

	return &tls.Config{
		RootCAs: certPool,
		CipherSuites: []uint16{
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},

		MinVersion: tls.VersionTLS13,
	}, nil
}
