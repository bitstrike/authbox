#!/bin/bash
# Filename: authbox-vm-client.sh
# Launch the authbox test VM (QEMU/KVM) with the YubiKey USB-forwarded
# so pam_u2f console login inside the guest can see the hardware key.
set -euo pipefail

YUBICO_VENDOR_ID="0x1050"   # Yubico.com
TAP_DEV="tap101"
BRIDGE_DEV="lanbr0"

# Tear down the tap device on exit (normal exit, error, or interrupt).
cleanup() {
  sudo ip link set dev "$TAP_DEV" down 2>/dev/null || true
  sudo ip tuntap del dev "$TAP_DEV" mode tap 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Bridged tap networking. Remove any stale device first so a leftover tap from
# a previous run does not cause "Device or resource busy".
sudo ip tuntap del dev "$TAP_DEV" mode tap 2>/dev/null || true
sudo ip tuntap add dev "$TAP_DEV" mode tap user "$USER"
sudo ip link set dev "$TAP_DEV" master "$BRIDGE_DEV"
sudo ip link set dev "$TAP_DEV" up

# Detect a Yubico device on the USB bus (any product under the Yubico vendor).
if ! lsusb | grep -iq '1050:'; then
  echo "WARN: no Yubico device found on USB bus. Console FIDO2 login will not work." >&2
  echo "      Plug the YubiKey into this host before launching." >&2
fi

qemu-system-x86_64 \
  -enable-kvm \
  -cpu host \
  -machine q35 \
  -smp 4 \
  -m 4G \
  -drive file="$PWD/vm-101-disk-0.qcow2",format=qcow2,if=virtio \
  -netdev tap,id=net0,ifname="$TAP_DEV",script=no,downscript=no \
  -device virtio-net-pci,netdev=net0,mac=52:54:00:01:01:01 \
  -usb \
  -device usb-host,vendorid="$YUBICO_VENDOR_ID"
