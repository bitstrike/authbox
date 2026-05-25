#!/bin/sh

# Use the `pass` utility to insert authbox items into your pass store.
# .env won't work for this and you'll commit it to github anyway so don't do it.

export LDAP_BASE_DN="$(pass show authbox/LDAP_BASE_DN)"
export INITIAL_ADMIN_EMAIL="$(pass show authbox/INITIAL_ADMIN_EMAIL)"
export TLS_DOMAIN="$(pass show authbox/TLS_DOMAIN)"
export TLS_ACME_EMAIL="$(pass show authbox/TLS_ACME_EMAIL)"
export AWS_HOSTED_ZONE_ID="$(pass show authbox/AWS_HOSTED_ZONE_ID)"
make run

