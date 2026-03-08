#!/usr/bin/env bash
set -euo pipefail

IFACE="nic0"
GATEWAY="192.168.8.1"

carrier="$(cat /sys/class/net/$IFACE/carrier 2>/dev/null || echo 0)"

if [[ "$carrier" != "1" ]] || ! ping -I "$IFACE" -c 3 -W 2 "$GATEWAY" >/dev/null 2>&1; then
  logger -t net-watchdog "Link failure on $IFACE, restarting interface"
  ip link set dev "$IFACE" down
  sleep 3
  ip link set dev "$IFACE" up
  ifreload -a
fi
