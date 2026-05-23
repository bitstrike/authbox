# Authbox

## Overview

A containerized platform providing centralized authentication and authorization for Linux systems. Authentication is delegated to external OIDC identity providers (Google, Entra ID). POSIX identity data is served from OpenLDAP. A Go application provides administration, API access, and SSH certificate authority services.

## Architecture

### Container (Single Image)

- **OpenLDAP** - POSIX identity store (users, posixGroups, groupOfNames)
- **Go application** - Web UI, TLS REST API, SSH CA, LDAP management
- **SQLite** - Application state, provisioning cache, OIDC provider config (refactorable to Postgres)

The Go app manages OpenLDAP's `cn=config` directly via LDAP protocol on localhost. No config file generation or restarts required for LDAP configuration changes.

### Scaling

- Stateless Go app designed for horizontal scaling behind a load balancer
- OpenLDAP multi-master replication for directory HA
- Single container for initial deployment, separable into multiple containers later
- Container role (primary or replica) determined by `ROLE` env var at deploy time
- Replica discovers primary via `PRIMARY_HOST` env var
- Same image for all containers - behavior differs based on configuration
- Minimum HA deployment: 2 containers (1 primary, 1 replica)
- Failover: admin promotes replica by changing `ROLE=primary` and restarting

## Identity Providers

- Google (OIDC) or Microsoft Entra ID (OIDC) - one active provider at a time, not both
- Platform configuration selects which IdP is active
- Platform federates to the configured IdP for all identity verification
- No local passwords managed by the platform for OIDC flows

## Authentication Flows

### SSH Access (Remote Hosts)

- Platform holds an SSH CA private key
- User authenticates via OIDC (browser flow) to the web UI or API
- User submits their public key, platform signs it and returns a short-lived SSH certificate
- Certificate encodes username as principal, has configurable TTL (e.g., 8-12h)
- All hosts trusting the CA accept the certificate - no per-host key management
- Any user can obtain a cert for any host; access control is handled by other mechanisms (firewall, groups, host-level policy)
- Works with `ssh -A` (agent forwarding) through bastion hosts
- Works with `ssh -J` (ProxyJump) for direct connections through bastions
- User's private key can live anywhere: YubiKey, 1Password, disk - platform only signs the public key
- MFA is enforced by the IdP (Google/Entra) during the OIDC flow at cert issuance time

### Desktop/GDM Login

- FIDO2 via `pam_u2f` (Yubico PAM module)
- YubiKey 5C NFC as hardware token
- PIN entry + physical touch to authenticate
- Fully offline - no network connectivity required at login time
- Key registration performed through the platform's web UI (OIDC-authenticated enrollment)
- Credential mappings (`/etc/u2f_mappings`) synced to hosts when online
- Key loss recovery: user boots a live Linux distro, connects to company VPN, contacts admin for re-enrollment

### API/Automation Access

- OAuth2 client credentials grant for headless/server-to-server use
- An admin creates a service account via the web UI (OIDC-authenticated)
- Platform generates a client_id and client_secret, shown once at creation time
- Admin stores the secret in the service's environment (env var, secrets manager, etc.)
- The service exchanges client_id + client_secret for a bearer token, no browser needed
- Scoped permissions (which principals/operations a service account can request)
- Used for: SSH cert issuance from scripts, CI/CD, admin automation

## API Roles

| Role | Access |
|---|---|
| self | Own user record, own certs, own FIDO2 credentials |
| viewer | Read-only access to all users, groups, certs |
| operator | Create/update/disable users, manage group membership (onboarding/offboarding) |
| admin | Full CRUD on all users, groups, certs, config, LDAP settings |
| system | Internal sync endpoints (container-to-container only) |

Roles are assigned to OAuth2 tokens (interactive users via OIDC claims, service accounts at creation time). All authenticated users implicitly have the "self" role.

## User Provisioning

- Users must be explicitly provisioned in LDAP by an admin or operator (no auto-creation on OIDC login)
- Provisioning can be done via web UI (admin/operator) or API (service account with operator or admin role)
- Bulk import supported via API for onboarding from external systems
- OIDC login is only permitted for users who already exist in the directory

### UID/GID Assignment

- uidNumber and gidNumber can be explicitly specified at user/group creation time
- If not specified, the platform auto-assigns the next available value from a configurable range
- API validates uniqueness on create and update - returns `409 Conflict` if the uidNumber or gidNumber is already in use
- Responses always include the assigned uidNumber/gidNumber (whether explicit or auto-assigned)
- UID/GID can be changed via the web UI or API (subject to the same uniqueness check)
- UIDs are never reused - deprovisioned users are disabled, not deleted

### User Deprovisioning

- Disabled users remain in LDAP with their uidNumber intact (so file ownership still resolves to a name)
- loginShell set to `/sbin/nologin`, removed from all groups, FIDO2 credentials revoked
- Account marked as disabled in the platform - not available for login
- uidNumber is never reassigned to another user

## API

HTTPS/TLS endpoint exposing all platform features for automation.

Authentication: OAuth2 bearer tokens (OIDC for interactive, client credentials for automation).

### Response Format

Success (list endpoints):
```json
{
  "data": [...],
  "pagination": {
    "offset": 0,
    "limit": 50,
    "total": 1243
  }
}
```

Error:
```json
{
  "error": {
    "code": "CONFLICT",
    "message": "uidNumber 5001 already assigned to user bdoe"
  }
}
```

List endpoints accept `?offset=0&limit=50` query parameters. Default limit is 50.

### Core Endpoints

| Endpoint | Purpose |
|---|---|
| `POST /api/v1/ssh/sign` | Sign a public key, return SSH certificate |
| `GET /api/v1/ssh/ca.pub` | Return CA public key (for host provisioning) |
| `GET /api/v1/ssh/certs` | List issued certificates (audit) |
| `POST /api/v1/users` | Create user in LDAP |
| `GET /api/v1/users` | List users |
| `PUT /api/v1/users/{uid}` | Update user |
| `DELETE /api/v1/users/{uid}` | Remove user |
| `POST /api/v1/groups` | Create posixGroup or groupOfNames |
| `GET /api/v1/groups` | List groups |
| `PUT /api/v1/groups/{cn}` | Update group membership |
| `DELETE /api/v1/groups/{cn}` | Remove group |
| `POST /api/v1/fido2/register` | Register a FIDO2 credential for a user |
| `GET /api/v1/fido2/credentials/{uid}` | Get credential mappings for a user |
| `GET /api/v1/config/export` | Export LDAP config and directory for backup |
| `POST /api/v1/config/import` | Import config to rebuild directory |

### Internal Sync Endpoints (container-to-container only)

| Endpoint | Purpose |
|---|---|
| `GET /internal/sync/state` | Returns current state version (sequence number) |
| `GET /internal/sync/changes?since={version}` | Returns changes since a given version (delta sync) |
| `GET /internal/sync/snapshot` | Full SQLite state for initial sync or recovery |

These endpoints are authenticated between containers (mutual TLS or shared secret) and not exposed to users or admins. Replicas connect on startup for a full snapshot, then poll for deltas. If a replica falls too far behind, it re-fetches the full snapshot.

## Web UI

- OIDC-authenticated admin interface
- User and group CRUD
- LDAP configuration management
- SSH CA management and cert issuance
- FIDO2 key enrollment (QR/registration flow)
- OIDC provider configuration
- Export/import for backup and disaster recovery

## OpenLDAP Directory

- POSIX attributes: uid, uidNumber, gidNumber, homeDirectory, loginShell
- posixGroup objects for primary group membership
- groupOfNames objects for role/access group membership
- Schema managed via `cn=config` by the Go app
- Replication configurable for multi-container deployments

## Host-Side Stack

Minimal dependencies on managed hosts:

- **`nslcd`** - NSS lookups against OpenLDAP (lightweight, simple, reliable)
- **`pam_u2f`** - FIDO2 authentication for GDM/desktop login
- **`sshd` with `TrustedUserCAKeys`** - Accepts SSH certificates signed by platform CA
- No SSSD

### Host Provisioning

- All host provisioning managed via Ansible (playbooks and roles under `ansible/` directory)
- CA public key deployed to `/etc/ssh/trusted_ca.pub`
- FIDO2 credential mappings synced to `/etc/u2f_mappings`
- `nslcd.conf` pointed at platform container's LDAP
- PAM stack configured for `pam_u2f`

## Hardware

- **YubiKey 5C NFC** - USB-C + NFC, FIDO2/U2F, PIV, OpenPGP, TOTP capable
- USB-C to USB-A adapter for desktops with only USB-A ports
- Two keys per user recommended (primary + backup)

## Technology Stack

- **Language**: Go
- **Directory**: OpenLDAP
- **Database**: SQLite (upgradeable to Postgres)
- **Container**: Single Docker image
- **Host PAM**: `pam_u2f` (Yubico), `nslcd` (PADL)

See [webstack.md](webstack.md) for web framework, libraries, and project layout details.

## Logging

- All API endpoint connections are logged
- Application logging via a log class with `info()`, `debug()`, `error()`, `warn()` methods
- Logs stored on persistent volume mount (`./logs:/app/logs`)
- Log rotation handled by the application's logging class

## TLS Certificate Management

- TLS cert can be manually provisioned via volume mount
- Or: platform accepts AWS credential mount for automated DNS-01 Let's Encrypt provisioning

## Security Model

- OIDC is the root of trust for identity
- SSH certs are short-lived - expiry replaces revocation
- FIDO2 keys are registered only during OIDC-authenticated sessions
- Private keys never leave user devices (YubiKey or local storage)
- Platform CA private key stored in persistent volume (backed up externally)
- API authenticated via OAuth2 (no API keys or basic auth)
- All external communication over TLS

## Container Initialization (First Boot)

On first boot (empty persistent volume detected), the Go app performs:

1. Initialize OpenLDAP `cn=config` (MDB backend, load modules, set root DN and password)
2. Apply LDAP schema from `ldif/schema.ldif` (volume-mounted, editable by admin before init)
3. Create base DN and organizational units (`ou=people`, `ou=groups`)
4. Set LDAP ACLs for the directory
5. Generate SSH CA keypair (ed25519), store private key in `/data/ca/`
6. Provision the initial admin user in LDAP (using `INITIAL_ADMIN_EMAIL` env var)
7. Initialize SQLite schema

The `ldif/schema.ldif` file is the single source of truth for directory schema. An admin can edit it and re-bootstrap the container to apply changes. A default template ships in the repo.

Subsequent boots detect existing data and skip initialization.

## Backup and Recovery

- API endpoint exports full LDAP directory and `cn=config`
- Export includes user FIDO2 credential mappings
- Import endpoint allows rebuilding directory on a fresh container
- CA private key backed up separately (encrypted, external storage)
- SQLite database included in export
