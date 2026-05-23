#!/bin/sh
set -e

# Start slapd with STARTTLS on 389 and LDAPS on 636 (plaintext disabled)
# External: 389 (STARTTLS required) and 636 (LDAPS)
# Internal: localhost plain LDAP for Go app only, ldapi for local socket
slapd -h "ldap://0.0.0.0:389/ ldap://127.0.0.1:3389/ ldaps://0.0.0.0:636/ ldapi:///" -F /etc/openldap/slapd.d

# Wait for slapd to be ready
for i in $(seq 1 30); do
    if ldapsearch -x -H ldap://127.0.0.1:3389 -b "" -s base namingContexts >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Start the Go application
exec /usr/local/bin/authbox
