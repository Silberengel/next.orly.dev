#!/usr/bin/env bash
# Pure Go build with purego - no CGO needed
# libsecp256k1 is loaded dynamically at runtime if available
export CGO_ENABLED=0
if [ -f "pkg/crypto/p8k/libsecp256k1.so" ]; then
    export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)/pkg/crypto/p8k"
fi

go mod tidy
go test ./...
cd pkg/crypto
go mod tidy
go test ./...
cd ../database
go mod tidy
go test ./...
cd ../encoders
go mod tidy
go test ./...
cd ../protocol
go mod tidy
go test ./...
cd ../utils
go mod tidy
go test ./...
cd ../acl
go mod tidy
go test ./...