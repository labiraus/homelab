#!/usr/bin/env bash
set -euo pipefail

ROUTE_IFACE="${ROUTE_IFACE:-$(ip route show default | awk '/default/ {print $5; exit}')}"
GATEWAY="${GATEWAY:-$(ip route show default | awk '/default/ {print $3; exit}')}"

if [[ -z "${ROUTE_IFACE}" || -z "${GATEWAY}" ]]; then
  logger -t net-watchdog "No default route found; skipping watchdog run"
  exit 0
fi

LINK_IFACE="${LINK_IFACE:-$ROUTE_IFACE}"
if [[ -d "/sys/class/net/$ROUTE_IFACE/brif" ]]; then
  bridge_members=(/sys/class/net/"$ROUTE_IFACE"/brif/*)
  if [[ -e "${bridge_members[0]}" ]]; then
    LINK_IFACE="$(basename "${bridge_members[0]}")"
  fi
fi

if [[ ! -d "/sys/class/net/$LINK_IFACE" ]]; then
  logger -t net-watchdog "Interface $LINK_IFACE not found; skipping watchdog run"
  exit 0
fi

# Some USB CDC-NCM dongles do not reliably pass bridged VM return traffic
# unless the uplink is put into explicit user-requested promiscuous mode.
ip link set dev "$LINK_IFACE" promisc on >/dev/null 2>&1 || \
  logger -t net-watchdog "Failed to enable promisc on uplink=$LINK_IFACE"

has_lower_up=0
if ip -o link show dev "$LINK_IFACE" | grep -q 'LOWER_UP'; then
  has_lower_up=1
fi

if [[ "$has_lower_up" != "1" ]] || ! ping -I "$ROUTE_IFACE" -c 3 -W 2 "$GATEWAY" >/dev/null 2>&1; then
  logger -t net-watchdog "Link failure on uplink=$LINK_IFACE route_if=$ROUTE_IFACE gateway=$GATEWAY; restarting uplink"
  ip link set dev "$LINK_IFACE" down
  sleep 3
  ip link set dev "$LINK_IFACE" up
  ip link set dev "$LINK_IFACE" promisc on >/dev/null 2>&1 || \
    logger -t net-watchdog "Failed to re-enable promisc after restart on uplink=$LINK_IFACE"
fi
