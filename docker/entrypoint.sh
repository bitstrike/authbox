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

# Obtain TLS certificate
if [ ! -f "$TLS_CERT" ]; then
    mkdir -p "$(dirname "$TLS_CERT")"
    if [ -n "${TLS_DOMAIN:-}" ]; then
        # Obtain Let's Encrypt cert via DNS-01 (blocks until complete)
        echo "Obtaining Let's Encrypt certificate for ${TLS_DOMAIN}..."
        if ! /usr/local/bin/authbox --obtain-cert; then
            echo "ERROR: failed to obtain LE certificate" >&2
            exit 1
        fi
        echo "Let's Encrypt certificate obtained"
    else
        # No domain configured - generate self-signed for dev/testing
        echo "Generating self-signed TLS certificate (no TLS_DOMAIN set)"
        if ! openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 \
            -keyout "$TLS_KEY" -out "$TLS_CERT" \
            -days 365 -nodes -subj "/CN=authbox" \
            -addext "subjectAltName=DNS:localhost,DNS:authbox,IP:127.0.0.1" 2>&1; then
            echo "ERROR: failed to generate TLS cert" >&2
            exit 1
        fi
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

# Check for staged restore (live-restore pattern)
RESTORE_DIR="/data/live-restore"
if [ -d "$RESTORE_DIR" ]; then
    echo "Live restore detected: creating pre-import safety backup"

    BACKUP_DIR="/data/backups"
    TIMESTAMP=$(date +%Y%m%d-%H%M%S)
    mkdir -p "$BACKUP_DIR"

    # Pre-import safety backup
    PRE_BACKUP_DIR="$BACKUP_DIR/pre-import-backup-$TIMESTAMP-directory.ldif"
    PRE_BACKUP_CFG="$BACKUP_DIR/pre-import-backup-$TIMESTAMP-config.ldif"

    if ! slapcat -F "$SLAPD_CONF_DIR" > "$PRE_BACKUP_DIR" 2>/dev/null; then
        echo "ERROR: pre-import backup failed (disk full?). Aborting restore." >&2
        rm -f "$PRE_BACKUP_DIR"
        # Leave restore dir in place so operator can investigate
        echo "Starting slapd with existing data unchanged."
    else
        # Also backup cn=config
        slapcat -F "$SLAPD_CONF_DIR" -b "cn=config" > "$PRE_BACKUP_CFG" 2>/dev/null

        echo "Pre-import backup saved to $BACKUP_DIR"
        echo "Wiping existing LDAP data and restoring from staged files"

        # Wipe existing data
        rm -rf "${SLAPD_DATA_DIR:?}"/*
        rm -rf "${SLAPD_CONF_DIR:?}"/*

        RESTORE_OK=true

        # Restore cn=config if present
        if [ -f "$RESTORE_DIR/config.ldif" ]; then
            echo "Restoring cn=config..."
            if ! slapadd -n 0 -F "$SLAPD_CONF_DIR" -l "$RESTORE_DIR/config.ldif" 2>&1; then
                echo "ERROR: cn=config restore failed." >&2
                RESTORE_OK=false
            fi
        fi

        # Restore directory data if present
        if [ "$RESTORE_OK" = true ] && [ -f "$RESTORE_DIR/directory.ldif" ]; then
            echo "Restoring directory data..."
            if ! slapadd -F "$SLAPD_CONF_DIR" -l "$RESTORE_DIR/directory.ldif" 2>&1; then
                echo "ERROR: directory restore failed." >&2
                RESTORE_OK=false
            fi
        fi

        if [ "$RESTORE_OK" = true ]; then
            # Fix permissions after restore
            chown -R ldap:ldap "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR"
            # Remove restore dir so it doesn't re-apply on next restart
            rm -rf "$RESTORE_DIR"
            echo "Live restore complete"
        else
            echo "Restore failed. Attempting rollback from pre-import backup..."
            rm -rf "${SLAPD_DATA_DIR:?}"/*
            rm -rf "${SLAPD_CONF_DIR:?}"/*

            ROLLBACK_OK=true
            if [ -f "$PRE_BACKUP_CFG" ]; then
                if ! slapadd -n 0 -F "$SLAPD_CONF_DIR" -l "$PRE_BACKUP_CFG" 2>&1; then
                    echo "ERROR: rollback of cn=config failed." >&2
                    ROLLBACK_OK=false
                fi
            fi
            if [ "$ROLLBACK_OK" = true ] && [ -f "$PRE_BACKUP_DIR" ]; then
                if ! slapadd -F "$SLAPD_CONF_DIR" -l "$PRE_BACKUP_DIR" 2>&1; then
                    echo "ERROR: rollback of directory failed." >&2
                    ROLLBACK_OK=false
                fi
            fi

            if [ "$ROLLBACK_OK" = true ]; then
                chown -R ldap:ldap "$SLAPD_CONF_DIR" "$SLAPD_DATA_DIR"
                echo "Rollback successful. Renaming staged files to prevent retry."
                mv "$RESTORE_DIR/directory.ldif" "$RESTORE_DIR/directory.ldif.failed" 2>/dev/null
                mv "$RESTORE_DIR/config.ldif" "$RESTORE_DIR/config.ldif.failed" 2>/dev/null
            else
                echo "CRITICAL: Rollback also failed. Investigate disk space, permissions, and LDIF validity." >&2
                echo "Pre-import backup files remain at: $PRE_BACKUP_DIR and $PRE_BACKUP_CFG" >&2
                echo "Staged restore files remain at: $RESTORE_DIR/" >&2
            fi
        fi
    fi
fi

# Start slapd
echo "Starting slapd..."
mkdir -p /var/run/openldap
chown ldap:ldap /var/run/openldap

SLAPD_URLS="ldap://0.0.0.0:389/ ldap://127.0.0.1:3389/ ldaps://0.0.0.0:636/"

slapd -u ldap -g ldap -h "$SLAPD_URLS" -F "$SLAPD_CONF_DIR" -d 0 2>&1 &
SLAPD_PID=$!
sleep 2

if ! kill -0 "$SLAPD_PID" 2>/dev/null; then
    echo "ERROR: slapd exited immediately. Trying with debug:" >&2
    slapd -u ldap -g ldap -h "ldap://0.0.0.0:389/ ldap://127.0.0.1:3389/" -F "$SLAPD_CONF_DIR" -d 1 2>&1 | head -50
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
