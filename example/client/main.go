// Client demonstrates a DTLS client that authenticates to a server using its
// SPIFFE X.509-SVID.
//
// Prerequisites: a SPIRE agent must be running and its Workload API socket
// must be reachable at the default path (or set SPIFFE_ENDPOINT_SOCKET).
//
// Run after example/server. The client sends a single datagram, prints the
// echoed response, then exits.
//
// Usage:
//
//	go run ./example/client
package main

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/pion/dtls/v2"
	dtlssvid "github.com/jsnctl/spiffe-dtls"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

const addr = "127.0.0.1:4444"

func main() {
	ctx := context.Background()

	source, err := workloadapi.NewX509Source(ctx)
	if err != nil {
		log.Fatalf("creating X.509 source: %v", err)
	}
	defer source.Close()

	// Print our own SVID so the demo output is self-explanatory.
	svid, err := source.GetX509SVID()
	if err != nil {
		log.Fatalf("getting own SVID: %v", err)
	}
	log.Printf("our SPIFFE ID: %s", svid.ID)

	cfg, err := dtlssvid.MTLSClientConfig(source, source, tlsconfig.AuthorizeAny())
	if err != nil {
		log.Fatalf("building DTLS client config: %v", err)
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		log.Fatalf("resolving address: %v", err)
	}

	conn, err := dtls.Dial("udp", udpAddr, cfg)
	if err != nil {
		log.Fatalf("dtls.Dial: %v", err)
	}
	defer conn.Close()

	if _, err := fmt.Fprintf(conn, "hello from SPIFFE-over-DTLS"); err != nil {
		log.Fatalf("write: %v", err)
	}

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		log.Fatalf("read: %v", err)
	}
	log.Printf("server response: %s", buf[:n])
}
