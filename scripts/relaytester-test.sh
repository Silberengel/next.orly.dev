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
export ORLY_LOG_LEVEL=trace
export ORLY_LOG_TO_STDOUT=true
export ORLY_LISTEN=127.0.0.1
export ORLY_PORT=3334
export ORLY_IP_WHITELIST=127.0.0
export ORLY_ADMINS=8118b9201de133912079652601863a69fdd0cac7f3eb15a38ae410c3f364269c,57eaff2aec61241dd8925b4d3a9cc93824a2944ed3f9e6b5143c15d0dd911864
export ORLY_ACL_MODE=none
go run . &
sleep 5
relay-tester ws://127.0.0.1:3334 nsec12l4072hvvyjpmkyjtdxn48xf8qj299zw60u7ddg58s2aphv3rpjqtg0tvr nsec1syvtjgqauyeezgrev5nqrp36d87apjk87043tgu2usgv8umyy6wq4yl6tu
killall next.orly.dev