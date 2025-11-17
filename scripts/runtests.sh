#!/usr/bin/env bash
# Pure Go build with purego - no CGO needed
# libsecp256k1 is loaded dynamically at runtime if available
export CGO_ENABLED=0
if [ -f "pkg/crypto/p8k/libsecp256k1.so" ]; then
    export LD_LIBRARY_PATH="${LD_LIBRARY_PATH:+$LD_LIBRARY_PATH:}$(pwd)/pkg/crypto/p8k"
fi
go test -v ./... -bench=. -run=xxx -benchmem