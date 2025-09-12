#!/bin/bash

# Setup script for downloading and configuring external relay repositories
# for benchmarking

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EXTERNAL_DIR="${SCRIPT_DIR}/external"

echo "Setting up external relay repositories for benchmarking..."

# Create external directory
mkdir -p "${EXTERNAL_DIR}"

# Function to clone or update repository
clone_or_update() {
    local repo_url="$1"
    local repo_dir="$2"
    local repo_name="$3"
    
    echo "Setting up ${repo_name}..."
    
    if [ -d "${repo_dir}" ]; then
        echo "  ${repo_name} already exists, updating..."
        cd "${repo_dir}"
        git pull origin main 2>/dev/null || git pull origin master 2>/dev/null || true
        cd - > /dev/null
    else
        echo "  Cloning ${repo_name}..."
        git clone "${repo_url}" "${repo_dir}"
    fi
}

# Clone khatru
clone_or_update "https://github.com/fiatjaf/khatru.git" "${EXTERNAL_DIR}/khatru" "Khatru"

# Clone relayer
clone_or_update "https://github.com/fiatjaf/relayer.git" "${EXTERNAL_DIR}/relayer" "Relayer"

# Clone strfry
clone_or_update "https://github.com/hoytech/strfry.git" "${EXTERNAL_DIR}/strfry" "Strfry"

# Clone nostr-rs-relay
clone_or_update "https://git.sr.ht/~gheartsfield/nostr-rs-relay" "${EXTERNAL_DIR}/nostr-rs-relay" "Nostr-rs-relay"

echo "Creating Dockerfiles for external relays..."

# Create Dockerfile for Khatru SQLite
cat > "${SCRIPT_DIR}/Dockerfile.khatru-sqlite" << 'EOF'
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates sqlite-dev gcc musl-dev

WORKDIR /build
COPY . .

# Build the basic-sqlite example
RUN cd examples/basic-sqlite && \
    go mod tidy && \
    CGO_ENABLED=1 go build -o khatru-sqlite .

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite wget
WORKDIR /app
COPY --from=builder /build/examples/basic-sqlite/khatru-sqlite /app/
RUN mkdir -p /data
EXPOSE 8080
ENV DATABASE_PATH=/data/khatru.db
ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080 || exit 1
CMD ["/app/khatru-sqlite"]
EOF

# Create Dockerfile for Khatru Badger
cat > "${SCRIPT_DIR}/Dockerfile.khatru-badger" << 'EOF'
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /build
COPY . .

# Build the basic-badger example
RUN cd examples/basic-badger && \
    go mod tidy && \
    CGO_ENABLED=0 go build -o khatru-badger .

FROM alpine:latest
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=builder /build/examples/basic-badger/khatru-badger /app/
RUN mkdir -p /data
EXPOSE 8080
ENV DATABASE_PATH=/data/badger
ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080 || exit 1
CMD ["/app/khatru-badger"]
EOF

# Create Dockerfile for Relayer basic example
cat > "${SCRIPT_DIR}/Dockerfile.relayer-basic" << 'EOF'
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates sqlite-dev gcc musl-dev

WORKDIR /build
COPY . .

# Build the basic example
RUN cd examples/basic && \
    go mod tidy && \
    CGO_ENABLED=1 go build -o relayer-basic .

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite wget
WORKDIR /app
COPY --from=builder /build/examples/basic/relayer-basic /app/
RUN mkdir -p /data
EXPOSE 8080
ENV DATABASE_PATH=/data/relayer.db
ENV PORT=8080
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080 || exit 1
CMD ["/app/relayer-basic"]
EOF

# Create Dockerfile for Strfry
cat > "${SCRIPT_DIR}/Dockerfile.strfry" << 'EOF'
FROM ubuntu:22.04 AS builder

ENV DEBIAN_FRONTEND=noninteractive

# Install build dependencies
RUN apt-get update && apt-get install -y \
    git \
    build-essential \
    liblmdb-dev \
    libsecp256k1-dev \
    pkg-config \
    libtool \
    autoconf \
    automake \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /build
COPY . .

# Build strfry
RUN make setup-golpe && \
    make -j$(nproc)

FROM ubuntu:22.04
RUN apt-get update && apt-get install -y \
    liblmdb0 \
    libsecp256k1-0 \
    curl \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /build/strfry /app/
RUN mkdir -p /data

EXPOSE 8080
ENV STRFRY_DB_PATH=/data/strfry.lmdb
ENV STRFRY_RELAY_PORT=8080

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:8080 || exit 1

CMD ["/app/strfry", "relay"]
EOF

# Create Dockerfile for nostr-rs-relay
cat > "${SCRIPT_DIR}/Dockerfile.nostr-rs-relay" << 'EOF'
FROM rust:1.70-alpine AS builder

RUN apk add --no-cache musl-dev sqlite-dev

WORKDIR /build
COPY . .

# Build the relay
RUN cargo build --release

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite wget
WORKDIR /app
COPY --from=builder /build/target/release/nostr-rs-relay /app/
RUN mkdir -p /data

EXPOSE 8080
ENV RUST_LOG=info

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD wget --quiet --tries=1 --spider http://localhost:8080 || exit 1

CMD ["/app/nostr-rs-relay"]
EOF

echo "Creating configuration files..."

# Create configs directory
mkdir -p "${SCRIPT_DIR}/configs"

# Create strfry configuration
cat > "${SCRIPT_DIR}/configs/strfry.conf" << 'EOF'
##
## Default strfry config
##

# Directory that contains the strfry LMDB database (restart required)
db = "/data/strfry.lmdb"

dbParams {
    # Maximum number of threads/processes that can simultaneously have LMDB transactions open (restart required)
    maxreaders = 256

    # Size of mmap to use when loading LMDB (default is 1TB, which is probably reasonable) (restart required)
    mapsize = 1099511627776
}

relay {
    # Interface to listen on. Use 0.0.0.0 to listen on all interfaces (restart required)
    bind = "0.0.0.0"

    # Port to open for the nostr websocket protocol (restart required)
    port = 8080

    # Set OS-limit on maximum number of open files/sockets (if 0, don't attempt to set) (restart required)
    nofiles = 1000000

    # HTTP header that contains the client's real IP, before reverse proxying (ie x-real-ip) (MUST be all lower-case)
    realIpHeader = ""

    info {
        # NIP-11: Name of this server. Short/descriptive (< 30 characters)
        name = "strfry benchmark"

        # NIP-11: Detailed description of this server, free-form
        description = "A strfry relay for benchmarking"

        # NIP-11: Administrative pubkey, for contact purposes
        pubkey = ""

        # NIP-11: Alternative contact for this server
        contact = ""
    }

    # Maximum accepted incoming websocket frame size (should be larger than max event) (restart required)
    maxWebsocketPayloadSize = 131072

    # Websocket-level PING message frequency (should be less than any reverse proxy idle timeouts) (restart required)
    autoPingSeconds = 55

    # If TCP keep-alive should be enabled (detect dropped connections to upstream reverse proxy) (restart required)
    enableTcpKeepalive = false

    # How much uninterrupted CPU time a REQ query should get during its DB scan
    queryTimesliceBudgetMicroseconds = 10000

    # Maximum records that can be returned per filter
    maxFilterLimit = 500

    # Maximum number of subscriptions (concurrent REQs) a connection can have open at any time
    maxSubsPerConnection = 20

    writePolicy {
        # If non-empty, path to an executable script that implements the writePolicy plugin logic
        plugin = ""
    }

    compression {
        # Use permessage-deflate compression if supported by client. Reduces bandwidth, but uses more CPU (restart required)
        enabled = true

        # Maintain a sliding window buffer for each connection. Improves compression, but uses more memory (restart required)
        slidingWindow = true
    }

    logging {
        # Dump all incoming messages
        dumpInAll = false

        # Dump all incoming EVENT messages
        dumpInEvents = false

        # Dump all incoming REQ/CLOSE messages
        dumpInReqs = false

        # Log performance metrics for initial REQ database scans
        dbScanPerf = false
    }

    numThreads {
        # Ingester threads: route incoming requests, validate events/sigs (restart required)
        ingester = 3

        # reqWorker threads: Handle initial DB scan for events (restart required)
        reqWorker = 3

        # reqMonitor threads: Handle filtering of new events (restart required)
        reqMonitor = 3

        # yesstr threads: experimental yesstr protocol (restart required)
        yesstr = 1
    }
}
EOF

# Create nostr-rs-relay configuration
cat > "${SCRIPT_DIR}/configs/config.toml" << 'EOF'
[info]
relay_url = "ws://localhost:8080"
name = "nostr-rs-relay benchmark"
description = "A nostr-rs-relay for benchmarking"
pubkey = ""
contact = ""

[database]
data_directory = "/data"
in_memory = false
engine = "sqlite"

[network]
port = 8080
address = "0.0.0.0"

[limits]
messages_per_sec = 0
subscriptions_per_min = 0
max_event_bytes = 65535
max_ws_message_bytes = 131072
max_ws_frame_bytes = 131072

[authorization]
pubkey_whitelist = []

[verified_users]
mode = "passive"
domain_whitelist = []
domain_blacklist = []

[pay_to_relay]
enabled = false

[options]
reject_future_seconds = 30
EOF

echo "Creating data directories..."
mkdir -p "${SCRIPT_DIR}/data"/{next-orly,khatru-sqlite,khatru-badger,relayer-basic,strfry,nostr-rs-relay}
mkdir -p "${SCRIPT_DIR}/reports"

echo "Setup complete!"
echo ""
echo "External relay repositories have been cloned to: ${EXTERNAL_DIR}"
echo "Dockerfiles have been created for all relay implementations"
echo "Configuration files have been created in: ${SCRIPT_DIR}/configs"
echo "Data directories have been created in: ${SCRIPT_DIR}/data"
echo ""
echo "To run the benchmark:"
echo "  cd ${SCRIPT_DIR}"
echo "  docker-compose up --build"
echo ""
echo "Reports will be generated in: ${SCRIPT_DIR}/reports"