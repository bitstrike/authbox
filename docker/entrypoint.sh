#!/bin/sh
set -e

# Start slapd on localhost only (Go app manages it via ldapi or localhost)
slapd -h "ldap://127.0.0.1:389/ ldapi:///" -F /etc/openldap/slapd.d

# Wait for slapd to be ready
for i in $(seq 1 30); do
    if ldapsearch -x -H ldap://127.0.0.1:389 -b "" -s base namingContexts >/dev/null 2>&1; then
        break
    fi
    sleep 1
done

# Start the Go application
exec /usr/local/bin/authbox
