#!/bin/sh

SLAPD_CONF_DIR="/etc/openldap/slapd.d"
SLAPD_DATA_DIR="/var/lib/openldap/openldap-data"
TLS_CERT="${TLS_CERT_PATH:-/data/tls/cert.pem}"
TLS_KEY="${TLS_KEY_PATH:-/data/tls/key.pem}"
LDAP_ADMIN_PASS="${LDAP_ADMIN_PASS:-admin}"

# Read LDAP admin password from secrets if available
if [ -f "/etc/secrets/authbox/ldap_admin_password" ]; then
    LDAP_ADMIN_PASS=$(cat /etc/secrets/authbox/ldap_admin_password | tr -d '\n')
fi

# Generate self-signed TLS cert if none exists
if [ ! -f "$TLS_CERT" ]; then
    echo "Generating self-signed TLS certificate"
    mkdir -p "$(dirname "$TLS_CERT")"
    if ! openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
        -keyout "$TLS_KEY" -out "$TLS_CERT" \
        -days 365 -nodes -subj "/CN=authbox" \
        -addext "subjectAltName=DNS:localhost,DNS:authbox,IP:127.0.0.1" 2>&1; then
        echo "ERROR: failed to generate TLS cert" >&2
        exit 1
    fi
    chmod 640 "$TLS_KEY"
    chown ldap:ldap "$TLS_CERT" "$TLS_KEY"
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
olcTLSCertificateFile: ${TLS_CERT}
olcTLSCertificateKeyFile: ${TLS_KEY}

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
    if ! slapadd -n 0 -F "$SLAPD_CONF_DIR" -l /tmp/init-config.ldif 2>&1; then
        echo "ERROR: slapadd failed" >&2
        exit 1
    fi
    rm /tmp/init-config.ldif

    chown -R ldap:ldap "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR" /var/run/openldap
fi

# Ensure permissions on subsequent boots
chown -R ldap:ldap "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR" /var/run/openldap

# Start slapd
echo "Starting slapd..."
slapd -u ldap -g ldap -h "ldap://0.0.0.0:389/ ldap://127.0.0.1:3389/ ldaps://0.0.0.0:636/ ldapi:///" -F "$SLAPD_CONF_DIR" -d 0 2>&1 &
SLAPD_PID=$!
sleep 2

if ! kill -0 "$SLAPD_PID" 2>/dev/null; then
    echo "ERROR: slapd exited immediately. Trying with debug:" >&2
    slapd -u ldap -g ldap -h "ldap://0.0.0.0:389/ ldap://127.0.0.1:3389/ ldapi:///" -F "$SLAPD_CONF_DIR" -d 1 2>&1 | head -50
    exit 1
fi

# Wait for slapd to be ready
echo "Waiting for slapd..."
for i in $(seq 1 30); do
    if ldapsearch -x -H ldap://127.0.0.1:3389 -b "" -s base namingContexts >/dev/null 2>&1; then
        echo "slapd ready"
        break
    fi
    if [ "$i" -eq 30 ]; then
        echo "ERROR: slapd not responding after 30s" >&2
        exit 1
    fi
    sleep 1
done

# Start the Go application
echo "Starting authbox..."
exec /usr/local/bin/authbox
