# Orly Web Application

This is a React web application that uses Bun for building and bundling, and is automatically embedded into the Go binary when built.

## Prerequisites

- [Bun](https://bun.sh/) - JavaScript runtime and toolkit
- Go 1.16+ (for embedding functionality)

## Development

To run the development server:

```bash
cd app/web
bun install
bun run dev
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