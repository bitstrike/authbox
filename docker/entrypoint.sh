#!/bin/sh
set -e

SLAPD_CONF_DIR="/etc/openldap/slapd.d"
SLAPD_DATA_DIR="/var/lib/openldap/openldap-data"
LDAP_ADMIN_PASS="${LDAP_ADMIN_PASS:-admin}"

# Read LDAP admin password from secrets if available
if [ -f "/etc/secrets/authbox/ldap_admin_password" ]; then
    LDAP_ADMIN_PASS=$(cat /etc/secrets/authbox/ldap_admin_password | tr -d '\n')
fi

# Bootstrap cn=config if slapd.d is empty (first boot)
if [ ! -f "$SLAPD_CONF_DIR/cn=config.ldif" ]; then
    echo "First boot: initializing slapd cn=config"

    HASHED_PASS=$(slappasswd -s "$LDAP_ADMIN_PASS")

    cat > /tmp/init-config.ldif <<EOF
dn: cn=config
objectClass: olcGlobal
cn: config
olcPidFile: /var/run/openldap/slapd.pid

dn: cn=schema,cn=config
objectClass: olcSchemaConfig
cn: schema

include: file:///etc/openldap/schema/core.ldif
include: file:///etc/openldap/schema/cosine.ldif
include: file:///etc/openldap/schema/inetorgperson.ldif
include: file:///etc/openldap/schema/nis.ldif

dn: olcDatabase={0}config,cn=config
objectClass: olcDatabaseConfig
olcDatabase: {0}config
olcRootDN: cn=admin,cn=config
olcRootPW: ${HASHED_PASS}

dn: cn=module{0},cn=config
objectClass: olcModuleList
cn: module{0}
olcModulePath: /usr/lib/openldap
olcModuleLoad: back_mdb

dn: olcDatabase={1}mdb,cn=config
objectClass: olcDatabaseConfig
objectClass: olcMdbConfig
olcDatabase: {1}mdb
olcSuffix: ${LDAP_BASE_DN:-dc=example,dc=com}
olcRootDN: cn=admin,${LDAP_BASE_DN:-dc=example,dc=com}
olcRootPW: ${HASHED_PASS}
olcDbDirectory: ${SLAPD_DATA_DIR}
olcDbMaxSize: 1073741824
olcDbIndex: objectClass eq
olcDbIndex: uid eq
olcDbIndex: uidNumber eq
olcDbIndex: gidNumber eq
olcDbIndex: cn eq
olcDbIndex: member eq
olcDbIndex: memberUid eq
EOF

    mkdir -p "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR"
    slapadd -n 0 -F "$SLAPD_CONF_DIR" -l /tmp/init-config.ldif
    rm /tmp/init-config.ldif

    # Fix permissions
    chown -R ldap:ldap "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR" /var/run/openldap
fi

# Ensure permissions on subsequent boots
chown -R ldap:ldap "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR" /var/run/openldap

# Start slapd
# External: 389 (STARTTLS), 636 (LDAPS)
# Internal: 3389 on localhost (plain, for Go app), ldapi for local socket
slapd -u ldap -g ldap -h "ldap://0.0.0.0:389/ ldap://127.0.0.1:3389/ ldaps://0.0.0.0:636/ ldapi:///" -F "$SLAPD_CONF_DIR"

# Wait for slapd to be ready
for i in $(seq 1 30); do
    if ldapsearch -x -H ldap://127.0.0.1:3389 -b "" -s base namingContexts >/dev/null 2>&1; then
        echo "slapd ready"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "slapd failed to start" >&2
        exit 1
    fi
    sleep 1
done

# Start the Go application
exec /usr/local/bin/authbox
