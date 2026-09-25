#!/bin/bash
# End-to-end check of 802.1Q trunk handling, verified with packet captures.
#
# Builds the namespace environment (scripts/env_setup.sh), runs gSwitch with
# config.toml, generates traffic across the h5 <-> sw5 trunk and then inspects
# tcpdump captures taken on the trunk (h5) and on the access ports (h1, h3).
#
# usage (from anywhere, as root):
#   sudo ./scripts/trunk_test.sh [path/to/gSwitch]
#
# The captures and switch log are kept in $OUT_DIR (default: a new /tmp dir).
# Requires: iproute2, tcpdump.

set -u
cd "$(dirname "$0")/.."

BIN=${1:-./gSwitch}
CONFIG=config.toml
OUT=${OUT_DIR:-$(mktemp -d /tmp/gswitch-trunk.XXXXXX)}
mkdir -p "$OUT"

# Switch MAC for VLAN 10 (etc/l2/ARPConfig.toml); routed frames leave with it.
VLAN10_GW_MAC=52:e1:47:de:21:2a

if [ "$(id -u)" -ne 0 ]; then
    echo "must be run as root" >&2
    exit 2
fi
if ! command -v tcpdump >/dev/null; then
    echo "tcpdump is required (apt-get install tcpdump)" >&2
    exit 2
fi
if [ ! -x "$BIN" ]; then
    echo "gSwitch binary not found at $BIN (build it with: go build)" >&2
    exit 2
fi

SW_PID=""
CAP_PIDS=()
cleanup() {
    [ ${#CAP_PIDS[@]} -gt 0 ] && kill "${CAP_PIDS[@]}" 2>/dev/null
    [ -n "$SW_PID" ] && kill "$SW_PID" 2>/dev/null
    wait 2>/dev/null
    ./scripts/env_destroy.sh >/dev/null 2>&1
}
trap cleanup EXIT

./scripts/env_destroy.sh >/dev/null 2>&1
./scripts/env_setup.sh >/dev/null || exit 1

echo "sw5 $(ip netns exec sw ethtool -k sw5 2>/dev/null | grep rx-vlan-offload)"

# -Z root: tcpdump drops to its own user by default and couldn't write to $OUT.
for host in h1 h3 h5; do
    ip netns exec "$host" tcpdump -i "$host" -Z root -U -nn -w "$OUT/$host.pcap" 2>/dev/null &
    CAP_PIDS+=($!)
done

ip netns exec sw "$BIN" "$CONFIG" >"$OUT/switch.log" 2>&1 &
SW_PID=$!

# Each port takes ~4s to come up; wait until every configured port is receiving.
PORTS=$(grep -c '^ *\[SwitchPorts\.' "$CONFIG")
echo "waiting for $PORTS switch ports to come up..."
for _ in $(seq 120); do
    [ "$(grep -c 'Staring RecvLoop' "$OUT/switch.log")" -ge "$PORTS" ] && break
    if ! kill -0 "$SW_PID" 2>/dev/null; then
        echo "gSwitch exited early, see $OUT/switch.log" >&2
        exit 1
    fi
    sleep 1
done
sleep 1

echo "generating traffic..."
ip netns exec h1 ping -c3 -W1 10.1.1.50 >"$OUT/ping_vlan1.txt" 2>&1
ip netns exec h3 ping -c3 -W1 10.10.1.50 >"$OUT/ping_vlan10.txt" 2>&1
ip netns exec h1 ping -c3 -W1 10.10.1.50 >"$OUT/ping_routed.txt" 2>&1
ip netns exec h5 ping -c3 -W1 10.20.1.1 >"$OUT/ping_vlan20.txt" 2>&1

sleep 1
kill -INT "${CAP_PIDS[@]}" 2>/dev/null
wait "${CAP_PIDS[@]}" 2>/dev/null
CAP_PIDS=()

for host in h1 h3 h5; do
    tcpdump -r "$OUT/$host.pcap" -e -nn -t 2>/dev/null >"$OUT/$host.txt"
done

FAILED=0
check() {
    local desc=$1
    shift
    if "$@"; then
        echo "PASS  $desc"
    else
        echo "FAIL  $desc"
        FAILED=1
    fi
}
# at_least <n> <file> <extended regex>
at_least() { [ "$(grep -cE "$3" "$2")" -ge "$1" ]; }
none() { ! grep -qE "$2" "$1"; }
ping_ok() { grep -q ' 0% packet loss' "$1"; }
ping_failed() { grep -q '100% packet loss' "$1"; }

echo
echo "--- VLAN 1 across the trunk (h1 <-> h5.1)"
check "h1 ping 10.1.1.50 succeeds" ping_ok "$OUT/ping_vlan1.txt"
check "trunk: requests to h5.1 carry tag 1" \
    at_least 3 "$OUT/h5.txt" 'vlan 1, .*10\.1\.1\.10 > 10\.1\.1\.50: ICMP echo request'
check "trunk: replies from h5.1 carry tag 1" \
    at_least 3 "$OUT/h5.txt" 'vlan 1, .*10\.1\.1\.50 > 10\.1\.1\.10: ICMP echo reply'

echo "--- VLAN 10 across the trunk (h3 <-> h5.10)"
check "h3 ping 10.10.1.50 succeeds" ping_ok "$OUT/ping_vlan10.txt"
check "trunk: requests to h5.10 carry tag 10" \
    at_least 3 "$OUT/h5.txt" 'vlan 10, .*10\.10\.1\.30 > 10\.10\.1\.50: ICMP echo request'
check "trunk: replies from h5.10 carry tag 10" \
    at_least 3 "$OUT/h5.txt" 'vlan 10, .*10\.10\.1\.50 > 10\.10\.1\.30: ICMP echo reply'

echo "--- routed VLAN 1 -> VLAN 10 onto the trunk (h1 -> h5.10)"
check "h1 ping 10.10.1.50 succeeds" ping_ok "$OUT/ping_routed.txt"
check "trunk: routed requests leave with tag 10 and the VLAN 10 gateway MAC" \
    at_least 3 "$OUT/h5.txt" "^$VLAN10_GW_MAC > .*vlan 10, .*10\.1\.1\.10 > 10\.10\.1\.50: ICMP echo request"

echo "--- access ports stay untagged"
check "h1 capture has no 802.1Q frames" none "$OUT/h1.txt" '802\.1Q'
check "h3 capture has no 802.1Q frames" none "$OUT/h3.txt" '802\.1Q'
check "h1 receives untagged replies from h5.1" \
    at_least 3 "$OUT/h1.txt" 'ethertype IPv4 .*10\.1\.1\.50 > 10\.1\.1\.10: ICMP echo reply'
check "h3 receives untagged replies from h5.10" \
    at_least 3 "$OUT/h3.txt" 'ethertype IPv4 .*10\.10\.1\.50 > 10\.10\.1\.30: ICMP echo reply'

echo "--- VLAN isolation"
check "no VLAN 10 ARP from h5.10 leaks onto h1 (VLAN 1)" none "$OUT/h1.txt" 'ARP.*tell 10\.10\.1\.50'
check "no VLAN 1 ARP from h5.1 leaks onto h3 (VLAN 10)" none "$OUT/h3.txt" 'ARP.*tell 10\.1\.1\.50'

echo "--- VLAN 20 is not allowed on the trunk"
check "h5.20 ping fails" ping_failed "$OUT/ping_vlan20.txt"
check "trunk: h5 sent VLAN 20 tagged ARP" at_least 1 "$OUT/h5.txt" 'vlan 20, .*ARP'
check "switch dropped VLAN 20 on sw5" at_least 1 "$OUT/switch.log" 'vlan id: 20 not allowed on port sw5'
check "no VLAN 20 traffic on h1 or h3" none <(cat "$OUT/h1.txt" "$OUT/h3.txt") '10\.20\.1\.'
check "gSwitch is still running" kill -0 "$SW_PID"

echo
echo "captures and logs: $OUT"
if [ "$FAILED" -ne 0 ]; then
    echo "TRUNK TEST FAILED"
    exit 1
fi
echo "TRUNK TEST PASSED"
