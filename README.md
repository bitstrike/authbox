# Authbox

<img src="images/authbox.png" alt="Authbox" width="40%">

Authbox is a centralized authentication and authorization container for Linux systems built on OpenLDAP and Go. A web frontend provides dashboard and management of users and groups through Google (tested) or Microsoft Entra ID (untested) OIDC authentication.

If not already obvious, there was a bit of AI assistance used to create this.
I'm still working out some of the details. Centralized password authentication on Linux isn't great. There are a lot of options but all of them have one drawback or another. Some options like SSSD work fine until they break for some reason. This project is intended to experiment with some other ways of maybe doing it. 

## Quick Start

```bash
# Build
docker compose -f docker/docker-compose.yml build primary

# Start (primary only)
docker compose -f docker/docker-compose.yml up primary
```

Web UI: `https://localhost:8443`

Without OIDC configured, the app runs in dev mode with auto-login as admin.

## Configuration

### Environment Variables

| Variable | Default | Description |
|---|---|---|
| `ROLE` | `primary` | Container role: `primary` or `replica` |
| `PRIMARY_HOST` | | Hostname of primary (replica only) |
| `RUNTIME_SECRETS` | `/etc/secrets/authbox` | Path to secrets directory |
| `LDAP_BASE_DN` | `dc=example,dc=com` | LDAP base distinguished name |
| `OIDC_ISSUER_URL` | | OIDC provider issuer URL |
| `OIDC_CLIENT_ID` | | OIDC client ID (fallback if not in secrets file) |
| `INITIAL_ADMIN_EMAIL` | | Email for first admin user (bootstrap) |
| `TLS_CERT_PATH` | `/data/tls/cert.pem` | Path to TLS certificate |
| `TLS_KEY_PATH` | `/data/tls/key.pem` | Path to TLS private key |
| `TLS_DOMAIN` | | Domain for Let's Encrypt (empty = self-signed) |
| `TLS_ACME_EMAIL` | | Contact email for ACME account |
| `AWS_HOSTED_ZONE_ID` | | Route53 hosted zone for DNS-01 challenges |
| `SSH_CERT_TTL` | `12h` | Default SSH certificate lifetime |
| `UID_RANGE_START` | `10000` | Start of auto-assigned UID range |
| `UID_RANGE_END` | `60000` | End of auto-assigned UID range |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `LOG_DIR` | `/app/logs` | Log output directory |

### Secrets Directory

Secrets are plain files on the **Docker host** at `/etc/secrets/authbox/`. The container bind-mounts this directory read-only (see `docker-compose.yml`).

Create the directory and populate it before starting the container:

```bash
sudo mkdir -p /etc/secrets/authbox
sudo chmod 750 /etc/secrets/authbox

# Create each file with appropriate content (see below)
sudo tee /etc/secrets/authbox/ldap_admin_password <<< "your-ldap-password"
sudo tee /etc/secrets/authbox/replica_sync_password <<< "your-sync-secret"
# ... google, entra, aws as needed

sudo chmod 640 /etc/secrets/authbox/*
```

Expected layout on the host:

```
/etc/secrets/authbox/
  aws                    - AWS credentials (optional, for Let's Encrypt DNS-01)
  google                 - Google OIDC credentials
  ldap_admin_password    - OpenLDAP admin bind password
  replica_sync_password  - Shared secret for replica sync
```

#### `/etc/secrets/authbox/google`

Google OIDC credentials. Key names follow Google's JSON credential format.

```
client_id=123456789-abc.apps.googleusercontent.com
client_secret=GOCSPX-xxxxx
```

#### `/etc/secrets/authbox/entra`

Microsoft Entra ID credentials. Key names follow Azure SDK convention. Use this instead of `google` (only one IdP active at a time).

```
AZURE_CLIENT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
AZURE_CLIENT_SECRET=xxxxxxxx
AZURE_TENANT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

#### `/etc/secrets/authbox/aws`

AWS credentials for Route53 DNS-01 Let's Encrypt automation. Key names follow `~/.aws/credentials` format.

```
aws_access_key_id=AKIAIOSFODNN7EXAMPLE
aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

#### `/etc/secrets/authbox/ldap_admin_password`

Single value. The OpenLDAP admin bind password.

#### `/etc/secrets/authbox/replica_sync_password`

Single value. Shared secret for container-to-container sync authentication.

### Volumes

| Path | Purpose |
|---|---|
| `/data` | SQLite database, SSH CA keys, TLS certs, exports |
| `/var/lib/openldap` | OpenLDAP MDB database |
| `/etc/openldap/slapd.d` | OpenLDAP cn=config |
| `/app/logs` | Application logs |
| `/app/ldif` | Schema LDIF (read-only, mounted from repo) |

## Ports

| Port | Protocol | Purpose |
|---|---|---|
| 389 | LDAP+STARTTLS | POSIX identity lookups (nslcd) |
| 636 | LDAPS | Legacy LDAP over TLS |
| 8443 | HTTPS | Web UI and REST API |

## Development

```bash
# Build binary
make build

# Run tests
make test

# Build and start container (clean first boot)
make run-clean

# Build and start (preserves volumes)
make run

# Stop
make stop

# Tail logs
make logs
```

## Testing LDAP

Verify STARTTLS on port 389 (use `LDAPTLS_REQCERT=never` for self-signed cert):

```bash
# Anonymous base search
LDAPTLS_REQCERT=never ldapsearch -ZZ -H ldap://localhost:389 -x \
  -b "dc=example,dc=com" -s base

# Authenticated search for users
LDAPTLS_REQCERT=never ldapsearch -ZZ -H ldap://localhost:389 -x \
  -D "cn=admin,dc=example,dc=com" \
  -w "$(sudo cat /etc/secrets/authbox/ldap_admin_password)" \
  -b "ou=people,dc=example,dc=com"

# Via LDAPS (port 636)
LDAPTLS_REQCERT=never ldapsearch -H ldaps://localhost:636 -x \
  -D "cn=admin,dc=example,dc=com" \
  -w "$(sudo cat /etc/secrets/authbox/ldap_admin_password)" \
  -b "ou=people,dc=example,dc=com"
```

Replace `dc=example,dc=com` with your `LDAP_BASE_DN`.

## Testing the API

```bash
# SSH CA public key (unauthenticated)
curl -sk https://localhost:8443/api/v1/ssh/ca.pub
```

## Architecture

See [project.md](project.md) for full architecture documentation.
See [webstack.md](webstack.md) for web framework details.
See [webui.md](webui.md) for UI page specifications.
