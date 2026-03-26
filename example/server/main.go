// Server demonstrates a DTLS server that accepts connections from workloads
// presenting SPIFFE X.509-SVIDs.
//
// Prerequisites: a SPIRE agent must be running and its Workload API socket
// must be reachable at the default path (or set SPIFFE_ENDPOINT_SOCKET).
//
// Run alongside example/client. The server authenticates the client's SVID and
// prints its SPIFFE ID to stdout for each accepted datagram.
//
// Usage:
//
//	go run ./example/server
package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"log"
	"net"

	dtlssvid "github.com/jsnctl/spiffe-dtls"
	"github.com/pion/dtls/v2"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

const addr = "127.0.0.1:4444"

func main() {
	ctx := context.Background()

	// X509Source connects to the SPIRE Workload API and watches for SVID
	// updates. It implements both x509svid.Source and x509bundle.Source, so
	// it can be passed directly to dtlssvid helpers.
	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("creating X.509 source: %v", err)
	}
	defer source.Close()

	// Accept any valid SPIFFE peer. Replace with tlsconfig.AuthorizeID or
	// tlsconfig.AuthorizeMemberOf to enforce a specific identity policy.
	cfg, err := dtlssvid.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
	if err != nil {
		log.Fatalf("building DTLS server config: %v", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("resolving address: %v", err)
	}

	listener, err := dtls.Listen("udp", udpAddr, cfg)
	if err != nil {
		log.Fatalf("dtls.Listen: %v", err)
	}
	defer listener.Close()

	log.Printf("listening on %s (DTLS/SPIFFE)", addr)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("accept error: %v", err)
			continue
		}
		go handle(conn)
	}
}

func handle(conn net.Conn) {
	defer conn.Close()

	dtlsConn, ok := conn.(*dtls.Conn)
	if !ok {
		log.Printf("unexpected connection type: %T", conn)
		return
	}

	// The peer SPIFFE ID is in the leaf certificate URI SAN, already verified
	// against the trust bundle by VerifyPeerCertificate in the handshake.
	// pion/dtls ConnectionState.PeerCertificates holds raw DER bytes; parse
	// them to access the URI SANs.
	state := dtlsConn.ConnectionState()
	if len(state.PeerCertificates) == 0 {
		log.Printf("no peer certificates (should not happen with RequireAnyClientCert)")
		return
	}
	leaf, err := x509.ParseCertificate(state.PeerCertificates[0])
	if err != nil {
		log.Printf("parsing peer leaf certificate: %v", err)
		return
	}
	spiffeID := "<none>"
	for _, uri := range leaf.URIs {
		if uri.Scheme == "spiffe" {
			spiffeID = uri.String()
			break
		}
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Printf("read error: %v", err)
		return
	}

	log.Printf("peer=%s msg=%q", spiffeID, buf[:n])
	fmt.Fprintf(conn, "echo: %s", buf[:n])
}
