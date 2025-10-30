#!/usr/bin/env bash
set -e

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )

# Update package lists
apt-get update

# Try to install from package manager first (much faster)
echo "Attempting to install secp256k1 from package manager..."
if apt-get install -y libsecp256k1-dev >/dev/null 2>&1; then
    echo "✓ Installed secp256k1 from package manager"
    exit 0
fi

# Fall back to building from source if package not available
echo "Package not available in repository, building from source..."

# Install build dependencies
apt-get install -y build-essential autoconf automake libtool git wget pkg-config

cd "$SCRIPT_DIR"
rm -rf secp256k1

# Clone and setup secp256k1
git clone https://github.com/bitcoin-core/secp256k1.git
cd secp256k1
git checkout v0.6.0

# Initialize and update submodules
git submodule init
git submodule update

# Build and install
./autogen.sh
./configure --enable-module-schnorrsig --enable-module-ecdh --prefix=/usr
make -j$(nproc)
make install

cd "$SCRIPT_DIR"
