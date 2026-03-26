spire_version := "1.14.4"
spire_bin     := ".spire/bin"
server_sock   := "/tmp/spire-server/private/api.sock"
agent_sock    := "/tmp/spire-agent/public/api.sock"

spire-install:
    #!/usr/bin/env bash
    set -euo pipefail
    if [ -x {{spire_bin}}/spire-server ]; then
        echo "SPIRE {{spire_version}} already installed."
        exit 0
    fi
    mkdir -p {{spire_bin}}
    ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')
    URL="https://github.com/spiffe/spire/releases/download/v{{spire_version}}/spire-{{spire_version}}-linux-${ARCH}-musl.tar.gz"
    echo "Downloading SPIRE {{spire_version}} (${ARCH})..."
    curl -fsSL "$URL" | tar -xz --strip-components=2 -C {{spire_bin}} \
        "spire-{{spire_version}}/bin/spire-server" \
        "spire-{{spire_version}}/bin/spire-agent"
    echo "Installed: $(ls {{spire_bin}})"

spire-up: spire-install
    #!/usr/bin/env bash
    set -euo pipefail
    
    # Clean up previous SPIRE
    fuser -k 8081/tcp 2>/dev/null || true
    rm -rf .spire/data
    mkdir -p .spire/data/server .spire/data/agent /tmp/spire-server/private /tmp/spire-agent/public

    echo "Starting SPIRE server..."
    {{spire_bin}}/spire-server run -config spire/server.conf &
    echo $! > .spire/server.pid

    echo "Waiting for SPIRE server..."
    for i in $(seq 30); do
        {{spire_bin}}/spire-server healthcheck -socketPath {{server_sock}} 2>/dev/null && break || sleep 1
    done

    echo "Generating join token..."
    TOKEN=$({{spire_bin}}/spire-server token generate \
        -socketPath {{server_sock}} \
        -spiffeID spiffe://example.org/agent | awk '{print $NF}')

    echo "Starting SPIRE agent..."
    {{spire_bin}}/spire-agent run -config spire/agent.conf -joinToken "$TOKEN" &
    echo $! > .spire/agent.pid

    echo "Waiting for agent socket..."
    for i in $(seq 30); do
        [ -S {{agent_sock}} ] && break || sleep 1
    done

    echo "Registering workload entry for uid=$(id -u)..."
    {{spire_bin}}/spire-server entry create \
        -socketPath {{server_sock}} \
        -spiffeID  spiffe://example.org/workload \
        -parentID  spiffe://example.org/agent \
        -selector  unix:uid:$(id -u)

    echo ""
    echo "SPIRE ready. Run 'just run' to spin up the DTLS demo workloads"

spire-down:
    #!/usr/bin/env bash
    for f in .spire/agent.pid .spire/server.pid; do
        if [ -f "$f" ]; then
            kill "$(cat $f)" 2>/dev/null || true
            rm "$f"
        fi
    done
    fuser -k 8081/tcp 2>/dev/null || true
    rm -f {{agent_sock}} {{server_sock}}
    echo "SPIRE stopped."

run:
    #!/usr/bin/env bash
    set -euo pipefail

    # Clean up previous run (if left behind)
    fuser -k 4444/udp 2>/dev/null || true

    go build -o ./bin/spiffe-dtls-server ./example/server
    go build -o ./bin/spiffe-dtls-client ./example/client
    export SPIFFE_ENDPOINT_SOCKET=unix://{{agent_sock}}
    ./bin/spiffe-dtls-server &
    SERVER_PID=$!
    echo "Waiting for server to bind (is SPIRE running?)..."
    until fuser 4444/udp 2>/dev/null; do sleep 0.2; done
    ./bin/spiffe-dtls-client
    kill $SERVER_PID 2>/dev/null || true
