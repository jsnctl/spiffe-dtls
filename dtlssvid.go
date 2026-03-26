// Package dtlssvid provides helpers for establishing mutually-authenticated
// DTLS connections using SPIFFE X.509-SVIDs as the credential source
//
// The API mirrors [github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig] so that
// callers already familiar with SPIFFE TLS helpers can adopt this DTLS approach
package dtlssvid

import (
	"crypto/tls"
	"fmt"

	"github.com/pion/dtls/v2"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

// MTLSClientConfig returns a [dtls.Config] for a DTLS client that presents its
// X.509-SVID and verifies the server's SVID against the provided bundle source.
//
// The authorizer is applied after chain verification. Use
// [tlsconfig.AuthorizeAny], [tlsconfig.AuthorizeID], or
// [tlsconfig.AuthorizeMemberOf] from go-spiffe.
func MTLSClientConfig(
	svid x509svid.Source,
	bundle x509bundle.Source,
	authorizer tlsconfig.Authorizer,
) (*dtls.Config, error) {
	return buildConfig(svid, bundle, authorizer, false)
}

// MTLSServerConfig returns a [dtls.Config] for a DTLS server that presents its
// X.509-SVID and requires mutual authentication from the client.
func MTLSServerConfig(
	svid x509svid.Source,
	bundle x509bundle.Source,
	authorizer tlsconfig.Authorizer,
) (*dtls.Config, error) {
	return buildConfig(svid, bundle, authorizer, true)
}

func buildConfig(
	svidSource x509svid.Source,
	bundleSource x509bundle.Source,
	authorizer tlsconfig.Authorizer,
	server bool,
) (*dtls.Config, error) {
	current, err := svidSource.GetX509SVID()
	if err != nil {
		return nil, fmt.Errorf("dtlssvid: fetching initial X.509-SVID: %w", err)
	}
	seed, err := svidToTLSCert(current)
	if err != nil {
		return nil, err
	}

	cfg := &dtls.Config{
		// Seed Certificates so pion has something to present if the callbacks
		// are invoked before the handshake begins. The callbacks below
		// supersede this for every real handshake.
		Certificates: []tls.Certificate{seed},

		// EMS provides additional protection against triple-handshake attacks.
		// Require it on both sides; older DTLS 1.2 stacks that do not support
		// EMS should be updated.
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,

		// SPIFFE verification replaces the standard pion-level TLS entirely
		// for this proof-of-concept implementation, so use InsecureSkipVerify
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: peerVerifier(bundleSource, authorizer),

		// Fetch a fresh SVID for every handshake, handling rotation
		// transparently. Note: pion/dtls defines its own ClientHelloInfo and
		// CertificateRequestInfo types rather than reusing crypto/tls
		GetCertificate: func(*dtls.ClientHelloInfo) (*tls.Certificate, error) {
			return getCert(svidSource)
		},
		GetClientCertificate: func(*dtls.CertificateRequestInfo) (*tls.Certificate, error) {
			return getCert(svidSource)
		},
	}

	if server {
		cfg.ClientAuth = dtls.RequireAnyClientCert
	}

	return cfg, nil
}

func getCert(source x509svid.Source) (*tls.Certificate, error) {
	svid, err := source.GetX509SVID()
	if err != nil {
		return nil, fmt.Errorf("dtlssvid: fetching X.509-SVID for handshake: %w", err)
	}
	cert, err := svidToTLSCert(svid)
	if err != nil {
		return nil, err
	}
	return &cert, nil
}

// svidToTLSCert converts the X509-SVID into the expected TLS cert
// serialisation required by the pion library for DLTS auth
func svidToTLSCert(svid *x509svid.SVID) (tls.Certificate, error) {
	// TODO: Error-handling
	chain := make([][]byte, len(svid.Certificates))
	for i, c := range svid.Certificates {
		chain[i] = c.Raw
	}
	return tls.Certificate{
		Certificate: chain,
		PrivateKey:  svid.PrivateKey,
		Leaf:        svid.Certificates[0],
	}, nil
}
