#!/bin/sh
# client-verify.sh - verify authbox enrollment on a client host
# Run on the enrolled machine after ansible provisioning

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
NC='\033[0m'

pass() { printf "${GREEN}PASS${NC} %s\n" "$1"; }
fail() { printf "${RED}FAIL${NC} %s\n" "$1"; }
warn() { printf "${YELLOW}WARN${NC} %s\n" "$1"; }

echo "=== Authbox Client Verification ==="
echo ""

# 1. nslcd running
echo "--- nslcd service ---"
if command -v rc-service >/dev/null 2>&1; then
  rc-service nslcd status >/dev/null 2>&1 && pass "nslcd running" || fail "nslcd not running"
elif command -v systemctl >/dev/null 2>&1; then
  systemctl is-active --quiet nslcd && pass "nslcd running" || fail "nslcd not running"
else
  pidof nslcd >/dev/null 2>&1 && pass "nslcd running" || fail "nslcd not running"
fi

# 2. nsswitch.conf
echo ""
echo "--- nsswitch.conf ---"
if grep -q 'ldap' /etc/nsswitch.conf 2>/dev/null; then
  pass "nsswitch.conf has ldap"
else
  fail "nsswitch.conf missing ldap"
fi

# 3. NSS resolution
echo ""
echo "--- NSS user resolution ---"
LDAP_USERS=$(getent passwd | grep -v '^\(root\|nobody\|daemon\|bin\|sys\|sync\|games\|man\|lp\|mail\|news\|uucp\)' | grep -c ':/home/')
if [ "$LDAP_USERS" -gt 0 ] 2>/dev/null; then
  pass "getent passwd returns $LDAP_USERS LDAP users"
else
  fail "getent passwd returns no LDAP users"
fi

echo ""
echo "--- NSS group resolution ---"
LDAP_GROUPS=$(getent group | grep -c ':[0-9]\{5\}:')
if [ "$LDAP_GROUPS" -gt 0 ] 2>/dev/null; then
  pass "getent group returns $LDAP_GROUPS LDAP groups"
else
  warn "getent group returns no high-GID groups (may be normal if no posixGroups exist)"
fi

# 4. LDAP network connectivity
echo ""
echo "--- LDAP connectivity ---"
LDAP_HOST=$(grep '^uri' /etc/nslcd.conf 2>/dev/null | awk '{print $2}' | sed 's|ldap[s]*://||' | sed 's|:.*||')
if [ -n "$LDAP_HOST" ]; then
  if nc -zw3 "$LDAP_HOST" 389 2>/dev/null; then
    pass "can reach $LDAP_HOST:389"
  else
    fail "cannot reach $LDAP_HOST:389"
  fi
else
  warn "could not parse LDAP host from /etc/nslcd.conf"
fi

# 5. SSH CA trust
echo ""
echo "--- SSH CA trust ---"
if [ -f /etc/ssh/trusted_ca.pub ]; then
  pass "/etc/ssh/trusted_ca.pub exists"
else
  fail "/etc/ssh/trusted_ca.pub missing"
fi

if grep -q 'TrustedUserCAKeys' /etc/ssh/sshd_config 2>/dev/null; then
  pass "sshd_config has TrustedUserCAKeys"
else
  fail "sshd_config missing TrustedUserCAKeys"
fi

# 6. pam_mkhomedir
echo ""
echo "--- pam_mkhomedir ---"
if grep -rq 'pam_mkhomedir' /etc/pam.d/ 2>/dev/null; then
  pass "pam_mkhomedir configured (home dirs created on first login)"
else
  fail "pam_mkhomedir not configured (LDAP users will have no home directory)"
fi

# 7. FIDO2 (optional)
echo ""
echo "--- FIDO2 (optional) ---"
if [ -f /etc/u2f_mappings ]; then
  KEYS=$(wc -l < /etc/u2f_mappings)
  pass "/etc/u2f_mappings exists ($KEYS entries)"
else
  warn "/etc/u2f_mappings not found (FIDO2 not configured)"
fi

echo ""
echo "=== Done ==="
