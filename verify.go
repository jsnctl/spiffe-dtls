package dtlssvid

import (
	"crypto/x509"

	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

func peerVerifier(
	bundle x509bundle.Source,
	authorizer tlsconfig.Authorizer,
) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, _ [][]*x509.Certificate) error {
		id, verifiedChains, err := x509svid.ParseAndVerify(rawCerts, bundle)
		if err != nil {
			return err
		}
		return authorizer(id, verifiedChains)
	}
}
