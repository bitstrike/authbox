#!/bin/bash
# test-ldap-tls.sh - integration test for LDAP transport security
#
# Codifies the manual checks used to probe auth.cloud.bitcrash.net:
#   - plaintext bind on 389 (currently honored; will be rejected once the
#     ssf=128 floor lands - see todo "Enforce STARTTLS / Minimum SSF on LDAP")
#   - STARTTLS on 389 (protocol + cert validation)
#   - LDAPS on 636 (protocol + cert validation)
#
# The script has two modes controlled by LDAP_ENFORCE_TLS:
#   unset/false (default) - pre-enforcement: plaintext 389 is EXPECTED to succeed.
#   true                  - post-enforcement: plaintext 389 is EXPECTED to be
#                           rejected (confidentialityRequired), STARTTLS/LDAPS
#                           are EXPECTED to succeed WITH cert validation.
#
# Usage:
#   LDAP_HOST=auth.cloud.bitcrash.net ./test-ldap-tls.sh
#   LDAP_HOST=auth.cloud.bitcrash.net LDAP_ENFORCE_TLS=true ./test-ldap-tls.sh
#
# Requires: ldapsearch, openssl.

set -u

LDAP_HOST="${LDAP_HOST:-auth.cloud.bitcrash.net}"
LDAP_PORT="${LDAP_PORT:-389}"
LDAPS_PORT="${LDAPS_PORT:-636}"
LDAP_BASE="${LDAP_BASE:-}"           # empty = root DSE base search
LDAP_ENFORCE_TLS="${LDAP_ENFORCE_TLS:-false}"
TIMEOUT="${TIMEOUT:-20}"

PASS=0
FAIL=0

ok()   { echo "  [PASS] $1"; PASS=$((PASS+1)); }
bad()  { echo "  [FAIL] $1"; FAIL=$((FAIL+1)); }
note() { echo "  - $1"; }

echo "=== LDAP Transport Security Test ==="
echo "Host: $LDAP_HOST  (ldap:$LDAP_PORT / ldaps:$LDAPS_PORT)"
echo "Mode: LDAP_ENFORCE_TLS=$LDAP_ENFORCE_TLS"
echo ""

# Preflight: required tools
for t in ldapsearch openssl; do
  command -v "$t" >/dev/null 2>&1 || { echo "MISSING required tool: $t"; exit 2; }
done

# Helper: run an anonymous base search, print exit code. Returns ldapsearch rc.
run_search() {
  # shellcheck disable=SC2086
  timeout "$TIMEOUT" ldapsearch "$@" -x -b "$LDAP_BASE" -s base -LLL namingContexts >/tmp/ldaptls.out 2>&1
  return $?
}

# ---------------------------------------------------------------------------
echo "[1/4] Plaintext bind on $LDAP_PORT"
run_search -H "ldap://$LDAP_HOST:$LDAP_PORT"
rc=$?
if [ "$LDAP_ENFORCE_TLS" = "true" ]; then
  # Expect rejection. ldapsearch rc 13 == confidentialityRequired.
  if [ "$rc" -ne 0 ] && grep -qi "confidential" /tmp/ldaptls.out; then
    ok "plaintext rejected (confidentialityRequired) - enforcement active"
  elif [ "$rc" -ne 0 ]; then
    ok "plaintext bind failed rc=$rc (rejected; verify reason below)"
    note "$(head -1 /tmp/ldaptls.out)"
  else
    bad "plaintext bind SUCCEEDED but enforcement expected it to be rejected"
  fi
else
  if [ "$rc" -eq 0 ]; then
    ok "plaintext bind succeeded (pre-enforcement baseline)"
    note "$(grep -i namingContexts /tmp/ldaptls.out | head -1)"
  else
    bad "plaintext bind failed rc=$rc (expected success pre-enforcement)"
    note "$(head -1 /tmp/ldaptls.out)"
  fi
fi
echo ""

# ---------------------------------------------------------------------------
echo "[2/4] STARTTLS on $LDAP_PORT with default cert validation (-ZZ)"
run_search -ZZ -H "ldap://$LDAP_HOST:$LDAP_PORT"
rc=$?
if [ "$rc" -eq 0 ]; then
  ok "STARTTLS succeeded with cert validation"
else
  if [ "$LDAP_ENFORCE_TLS" = "true" ]; then
    bad "STARTTLS FAILED with validation rc=$rc - clients would be locked out post-enforcement"
  else
    bad "STARTTLS failed with validation rc=$rc (investigate: cert trust / CN / chain)"
  fi
  note "$(head -2 /tmp/ldaptls.out | tr '\n' ' ')"
fi
echo ""

# ---------------------------------------------------------------------------
echo "[3/4] STARTTLS on $LDAP_PORT with validation disabled (isolates protocol vs trust)"
LDAPTLS_REQCERT=never run_search -ZZ -H "ldap://$LDAP_HOST:$LDAP_PORT"
rc=$?
if [ "$rc" -eq 0 ]; then
  ok "STARTTLS protocol works (REQCERT=never)"
  note "if [2] failed but this passed, the issue is cert VALIDATION, not the protocol"
else
  bad "STARTTLS failed even with REQCERT=never rc=$rc - protocol-level problem"
  note "$(head -2 /tmp/ldaptls.out | tr '\n' ' ')"
fi
echo ""

# ---------------------------------------------------------------------------
echo "[4/4] LDAPS on $LDAPS_PORT (handshake + cert verification via openssl)"
if timeout "$TIMEOUT" openssl s_client -connect "$LDAP_HOST:$LDAPS_PORT" -brief </dev/null >/tmp/ldaps.out 2>&1; then
  proto=$(grep -i "Protocol version" /tmp/ldaps.out | head -1 | sed 's/^ *//')
  cipher=$(grep -i "Ciphersuite" /tmp/ldaps.out | head -1 | sed 's/^ *//')
  cn=$(grep -i "Peer certificate" /tmp/ldaps.out | head -1 | sed 's/^ *//')
  verify=$(grep -i "Verification" /tmp/ldaps.out | head -1 | sed 's/^ *//')
  ok "LDAPS handshake established"
  note "$proto"; note "$cipher"; note "$cn"; note "$verify"
  if echo "$verify" | grep -qi "Verification: OK"; then
    ok "LDAPS cert verified against trust store"
  else
    bad "LDAPS cert did NOT verify: $verify"
  fi
else
  bad "LDAPS handshake/connection failed (port closed, filtered, or TLS error)"
  note "$(grep -iE 'connect|error|verify' /tmp/ldaps.out | head -2 | tr '\n' ' ')"
fi
echo ""

# ---------------------------------------------------------------------------
rm -f /tmp/ldaptls.out /tmp/ldaps.out
echo "=== Summary: $PASS passed, $FAIL failed ==="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
