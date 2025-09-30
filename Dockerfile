# Dockerfile for Stella's Nostr Relay (next.orly.dev)
# Owner: npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx

FROM golang:alpine AS builder

# Install build dependencies
RUN apk add --no-cache \
    git \
    build-base \
    autoconf \
    automake \
    libtool \
    pkgconfig

# Install secp256k1 library from Alpine packages
RUN apk add --no-cache libsecp256k1-dev

# Set working directory
WORKDIR /build

# Copy go modules first (for better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the relay with optimizations from v0.4.8
RUN CGO_ENABLED=1 GOOS=linux go build -ldflags "-w -s" -o relay .

# Create non-root user for security
RUN adduser -D -u 1000 stella && \
    chown -R 1000:1000 /build

# Final stage - minimal runtime image
FROM alpine:latest

# Install only runtime dependencies
RUN apk add --no-cache \
    ca-certificates \
    curl \
    libsecp256k1 \
    libsecp256k1-dev

WORKDIR /app

# Copy binary from builder
COPY --from=builder /build/relay /app/relay

# Create runtime user and directories
RUN adduser -D -u 1000 stella && \
    mkdir -p /data /profiles /app && \
    chown -R 1000:1000 /data /profiles /app

# Expose the relay port
EXPOSE 7777

# Set environment variables for Stella's relay
ENV ORLY_DATA_DIR=/data
ENV ORLY_LISTEN=0.0.0.0
ENV ORLY_PORT=7777
ENV ORLY_LOG_LEVEL=info
ENV ORLY_MAX_CONNECTIONS=1000
ENV ORLY_OWNERS=npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx
ENV ORLY_ADMINS=npub1v30tsz9vw6ylpz63g0a702nj3xa26t3m7p5us8f2y2sd8v6cnsvq465zjx,npub1l5sga6xg72phsz5422ykujprejwud075ggrr3z2hwyrfgr7eylqstegx9z

# Health check to ensure relay is responding
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD sh -c "code=\$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:7777 || echo 000); echo \$code | grep -E '^(101|200|400|404|426)$' >/dev/null || exit 1"

# Create volume for persistent data
VOLUME ["/data"]

# Drop privileges and run as stella user
USER 1000:1000

# Run Stella's Nostr relay
CMD ["/app/relay"]
