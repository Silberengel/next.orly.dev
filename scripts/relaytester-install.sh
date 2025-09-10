#!/usr/bin/env bash
## rust must be installed
if ! command -v "cargo" &> /dev/null; then
    echo "rust and cargo is not installed."
    echo "run this command to install:"
    echo
    echo "curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh"
    exit
else
    echo "cargo is installed."
fi

rm -rf relay-tester
git clone https://github.com/mikedilger/relay-tester.git
cd relay-tester
cargo build -r
cp target/release/relay-tester $GOBIN/
cd ..
#rm -rf relay-tester