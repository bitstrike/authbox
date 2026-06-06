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
- Key registration: user runs `pamu2fcfg` on their workstation, pastes credential string into web UI
- Platform stores credential, Ansible syncs to `/etc/u2f_mappings` on hosts
- Key loss recovery: user boots a live Linux distro, connects to company VPN, contacts admin for re-enrollment

### API/Automation Access

- OAuth2 client credentials grant for headless/server-to-server use
- An admin creates a service account via the web UI (OIDC-authenticated)
- Platform generates a client_id and client_secret, shown once at creation time
- client_secret is stored as a bcrypt hash (never retrievable after creation)
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

Roles are determined by LDAP group membership (groupOfNames):
- `cn=authbox-viewers,ou=groups` -> viewer
- `cn=authbox-operators,ou=groups` -> operator
- `cn=authbox-admins,ou=groups` -> admin

The initial admin user (from `INITIAL_ADMIN_EMAIL`) is added to `cn=authbox-admins` at bootstrap and has full access to all web UI and API features. All authenticated users implicitly have the "self" role. Google OIDC group claims are not used (unavailable without special account types).

## User Provisioning

- Users must be explicitly provisioned in LDAP by an admin or operator (no auto-creation on OIDC login)
- Provisioning can be done via web UI (admin/operator) or API (service account with operator or admin role)
- Bulk import supported via API and web UI (CSV and JSON formats)
- OIDC login is only permitted for users who already exist in the directory

### Bulk Import Format

CSV columns (header row required, phone columns optional):

```
uid,givenName,sn,mail,uidNumber,gidNumber,homeDirectory,loginShell,employeeType,telephoneNumber,mobile,homePhone,fax,pager
```

- For contact-type entries: leave uidNumber, gidNumber, homeDirectory empty. Shell is forced to `/sbin/nologin`.
- For posix users: UID/GID must be within the configured range or the import is rejected.
- Empty phone fields are valid (attributes not written to LDAP).
- If any row fails validation, the entire import is aborted (no partial imports).

### UID/GID Assignment

- uidNumber and gidNumber can be explicitly specified at user/group creation time
- If not specified, the platform auto-assigns the next available value from a configurable range
- API validates uniqueness on create and update - returns `409 Conflict` if the uidNumber or gidNumber is already in use
- Responses always include the assigned uidNumber/gidNumber (whether explicit or auto-assigned)
- UID/GID can be changed via the web UI or API (subject to the same uniqueness check)
- Preferred workflow: disable users rather than delete (preserves file ownership resolution)

### Employee Types

Users are classified by `employeeType` attribute (stored in LDAP, managed via Settings UI):

| Type | Emoji | Description |
|---|---|---|
| employee | 👤 | Standard user with posixAccount (UID/GID, shell, home directory) |
| contractor | 👷 | External user with posixAccount |
| service | 🤖 | Service/system account with posixAccount |
| contact | 🪪 | Directory-only entry (inetOrgPerson without posixAccount) |

Employee types are configurable via the Settings page (add/remove/reorder). The type determines:
- **contact**: no posixAccount objectClass, no UID/GID/shell/home directory. Cannot log in. Used for address book entries.
- **All others**: full posixAccount with UID/GID, home directory, and login shell.

### Phone Number Attributes

Standard inetOrgPerson phone attributes are supported for all user types:

| Field | LDAP Attribute | Description |
|---|---|---|
| Work Phone | telephoneNumber | Business phone |
| Mobile | mobile | Cell phone |
| Home Phone | homePhone | Home phone |
| Fax | facsimileTelephoneNumber | Fax number |
| Pager | pager | Pager number |

All phone fields are optional. Empty values are not written to LDAP. Clearing a phone field on update removes the attribute from the entry.

### User Deprovisioning

- Disabled users remain in LDAP with their uidNumber intact (so file ownership still resolves to a name)
- loginShell set to `/sbin/nologin`, removed from all groups, FIDO2 credentials revoked
- Account marked as disabled in the platform - not available for login
- uidNumber is not reassigned while the user entry exists

### User Deletion

- Admin-only action, only available on already-disabled accounts (two-step: disable first, then delete)
- Permanently removes the user entry from the LDAP directory
- The UID/GID becomes available for reassignment after deletion
- Warning presented: if the user owned files on any host, a future user assigned the same UID will inherit ownership of those files
- Requires "yesiagree" confirmation
- Use disable instead of delete when the user may have files on managed hosts
- Contacts (employeeType=contact) can be deleted without disabling first (they have no login capability)

### Safety Guards

- **Self-protection**: Users cannot delete or disable their own account (enforced at API and frontend handler level). Bulk operations silently skip the acting user.
- **Last-admin protection**: The last active member of `authbox-admins` cannot be deleted or disabled. Both single and bulk operations enforce this check by querying group membership and counting non-disabled admins.

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
| `POST /oauth/token` | Client credentials grant (service account token issuance) |
| `POST /api/v1/ssh/sign` | Sign a public key, return SSH certificate |
| `GET /api/v1/ssh/ca.pub` | Return CA public key (unauthenticated, also used as health check) |
| `GET /api/v1/ssh/certs` | List issued certificates (audit) |
| `POST /api/v1/users` | Create user in LDAP |
| `GET /api/v1/users` | List users |
| `PUT /api/v1/users/{uid}` | Update user |
| `POST /api/v1/users/{uid}/disable` | Disable user |
| `POST /api/v1/users/{uid}/enable` | Re-enable user (admin only) |
| `DELETE /api/v1/users/{uid}` | Delete user (admin only, must be disabled first) |
| `POST /api/v1/users/import` | Bulk import users (CSV or JSON) |
| `POST /api/v1/groups` | Create posixGroup or groupOfNames |
| `GET /api/v1/groups` | List groups |
| `PUT /api/v1/groups/{cn}` | Update group membership |
| `DELETE /api/v1/groups/{cn}` | Remove group |
| `POST /api/v1/fido2/register` | Register a FIDO2 credential for a user |
| `GET /api/v1/fido2/credentials/{uid}` | Get credential mappings for a user |
| `GET /api/v1/fido2/credentials` | All mappings in pam_u2f format (for Ansible sync) |
| `GET /api/v1/config/export` | Export LDAP config and directory for backup |
| `POST /api/v1/config/import` | Stage restore and restart container (returns `restart: true`) |

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
- FIDO2 key enrollment (user pastes `pamu2fcfg` output)
- OIDC provider configuration
- Export/import for backup and disaster recovery

### Bulk Actions

Tables with selectable rows (users, groups, SSH certs, FIDO2 keys, service accounts) support bulk operations via a shared `TableRenderer` component. Features:

- **Selection bar**: Shows count of selected items with available action buttons
- **Two-phase eligibility confirmation**: Actions with preconditions (e.g., delete requires disabled status) use a `EligibleIf` expression evaluated client-side against row data attributes. First click highlights ineligible rows in amber and shows the eligible count. Second click confirms.
- **Row data attributes**: User rows carry `data-disabled`, `data-type`, `data-self`, and `data-admin` attributes for client-side eligibility evaluation.
- **Conflict highlighting**: Ineligible rows receive a `conflict-row-bg` class (amber tint in both light and dark themes) so the admin can visually identify problematic selections before confirming.
- **Backend safety net**: Even if the frontend allows submission, backend handlers independently validate eligibility and skip ineligible items, reporting counts in the response.

See [webui.md](webui.md) for full page-by-page interface documentation.

## OpenLDAP Directory

- POSIX attributes: uid, uidNumber, gidNumber, homeDirectory, loginShell
- posixGroup objects for primary group membership
- groupOfNames objects for role/access group membership
- Schema managed via `cn=config` by the Go app
- Replication configurable for multi-container deployments
- Port 389 exposed externally with mandatory STARTTLS (plaintext rejected)
- Port 636 LDAPS supported for legacy applications
- Internal localhost port (3389) for Go app communication (plain, not exposed externally)

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
- Log rotation: daily, 90-day default retention
- Rotation and retention configurable via web UI

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

- Export via web UI or API produces a gzipped tar archive (LDAP directory LDIF, cn=config LDIF, SQLite state)
- Export includes FIDO2 credential mappings, service accounts, and SSH cert audit records
- CA private key is NOT included in exports (backed up separately, encrypted, external storage)
- Web UI supports scheduling daily `slapcat` backups with configurable retention

### Restore (live-restore pattern)

Import uses a staged file approach for safe LDAP restoration:

1. Web UI (or CLI) writes extracted LDIFs to `/data/live-restore/`
2. SQLite application state is restored immediately
3. Container restarts (process exits, Docker restart policy brings it back)
4. On startup, entrypoint detects `/data/live-restore/` and:
   - Runs `slapcat` to create a pre-import safety backup in `/data/backups/`
   - If pre-import backup fails: aborts import (likely disk full), starts slapd with existing data
   - If pre-import backup succeeds: wipes MDB and cn=config, runs `slapadd` with staged LDIFs
   - If slapadd fails: attempts rollback from the pre-import backup
   - If rollback also fails: logs critical error, operator must investigate
   - On success: removes `/data/live-restore/`, starts slapd normally

### CLI Restore (escape hatch)

An admin can manually place `directory.ldif` and/or `config.ldif` in `/data/live-restore/` on the persistent volume and restart the container. The entrypoint applies the same restore logic without needing the web UI.

## Design Decisions

### No SSSD
SSSD was rejected due to widespread reliability issues (cache corruption, silent failures, complex configuration). `nslcd` is simpler, lighter, and more predictable.

### No CLI Tool
A custom CLI tool for SSH cert retrieval was rejected as feature creep. Users authenticate via browser (OIDC), download cert from web UI or use a simple curl/script against the API.

### No LiteFS
LiteFS (FUSE-based SQLite replication) was rejected because FUSE requires `SYS_ADMIN` capability, violating least-privilege container security. API-based sync is used instead.

### FIDO2 Over TOTP for Desktop
FIDO2 (YubiKey) chosen over TOTP for GDM login because it works fully offline without cached credentials or network calls. TOTP would require either local secret storage or API validation.

### SQLite Over Postgres (Initial)
SQLite keeps the stack to two components (Go app + OpenLDAP). Postgres adds a third service to manage. The data access layer uses interfaces so the swap is a code change when needed.

### Single IdP at a Time
Supporting both Google and Entra simultaneously adds identity mapping complexity. One active IdP keeps user identity unambiguous.

## UI Components

Custom utility CSS (`internal/web/frontend/static/style.css`) with no build step. Dark mode via `.dark` class on `<html>`.

### CSS Component Classes

- `.card` - white box with border, rounded corners, padding. Dark mode variant.
- `.btn` + `.btn-primary` / `.btn-secondary` / `.btn-danger` - buttons with hover states
- `.badge` + `.badge-blue` / `.badge-purple` - pill labels (e.g., group type indicator)
- `.flash` + `.flash-success` / `.flash-error` / `.flash-warning` - notification bars
- `.input` - form inputs with focus ring and disabled state
- `.label` - form field labels
- `.table` - bordered table with header and row styling
- `.nav-link` - top navigation links with active state
- `.sidebar-nav-item` - sidebar navigation links with active/hover states
- `.status-dot` + `.green` / `.yellow` / `.red` - colored circle indicators
- `.bulk-bar` - bulk action toolbar (shown when rows selected)
- `.detail-layout` - responsive 2-column grid (form left, info panels right)

### Go Render Helpers

- `TableRenderer` / `TableConfig` - sortable, filterable, paginated tables with bulk actions and HTMX partials
- `SidebarRenderer` / `SidebarConfig` - two-column layout with nav links and HTMX content panel
- `flash.Set(w, type, message)` / `flash.Get(w, r)` - server-side one-time flash messages (cookie-based)
- `pageDataFromRequest(w, r, title, content)` - base page data builder (populates nav, role bools, flash)
