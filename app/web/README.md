# Orly Web Application

This is a React web application that uses Bun for building and bundling, and is automatically embedded into the Go binary when built.

## Prerequisites

- [Bun](https://bun.sh/) - JavaScript runtime and toolkit
- Go 1.16+ (for embedding functionality)

## Development

There are two ways to develop the web app:

1) Standalone (recommended for hot reload)
- Start the Go relay with the embedded web UI disabled so the React app can run on its own dev server with HMR.
- Configure the relay via environment variables:

```bash
# In another shell at repo root
export ORLY_WEB_DISABLE=true
# Optional: if you want same-origin URLs, you can set a proxy target and access the relay on the same port
# export ORLY_WEB_DEV_PROXY_URL=http://localhost:5173

# Start the relay as usual
go run .
```

- Then start the React dev server:

```bash
cd app/web
bun install
bun dev
```

When ORLY_WEB_DISABLE=true is set, the Go server still serves the API and websocket endpoints and sends permissive CORS headers, so the dev server can access them cross-origin. If ORLY_WEB_DEV_PROXY_URL is set, the Go server will reverse-proxy non-/api paths to the dev server so you can use the same origin.

2) Embedded (no hot reload)
- Build the web app and run the Go server with defaults:

```bash
cd app/web
bun install
bun run build
cd ../../
go run .
```

## Building

The React application needs to be built before compiling the Go binary to ensure that the embedded files are available:

```bash
# Build the React application
cd app/web
bun install
bun run build

# Build the Go binary from project root
cd ../../
go build
```

## How it works

1. The React application is built to the `app/web/dist` directory
2. The Go embed directive in `app/web.go` embeds these files into the binary
3. When the server runs, it serves the embedded React app at the root path

## Build Automation

You can create a shell script to automate the build process:

```bash
#!/bin/bash
# build.sh
echo "Building React app..."
cd app/web
bun install
bun run build

echo "Building Go binary..."
cd ../../
go build

echo "Build complete!"
```

Make it executable with `chmod +x build.sh` and run with `./build.sh`.