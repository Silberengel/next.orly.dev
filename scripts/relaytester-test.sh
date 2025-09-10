#!/usr/bin/env bash
## relay-tester must be installed
if ! command -v "relay-tester" &> /dev/null; then
    echo "relay-tester is not installed."
    echo "run this command to install:"
    echo
    echo "./scripts/relaytester-install.sh"
    exit
fi
rm -rf ~/.local/share/ORLY
export ORLY_LOG_LEVEL=off
export ORLY_LISTEN=127.0.0.1
export ORLY_PORT=3334
export ORLY_IP_WHITELIST=127.0.0
export ORLY_ADMINS=nsec12l4072hvvyjpmkyjtdxn48xf8qj299zw60u7ddg58s2aphv3rpjqtg0tvr,nsec1syvtjgqauyeezgrev5nqrp36d87apjk87043tgu2usgv8umyy6wq4yl6tu
go run . &
sleep 2
relay-tester ws://127.0.0.1:3334 nsec12l4072hvvyjpmkyjtdxn48xf8qj299zw60u7ddg58s2aphv3rpjqtg0tvr nsec1syvtjgqauyeezgrev5nqrp36d87apjk87043tgu2usgv8umyy6wq4yl6tu
killall next.orly.dev