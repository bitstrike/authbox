# Authbox - Implementation Checklist

## Phase 1: Project Scaffolding

- [x] Initialize Go module (`go mod init`)
- [x] Create project directory structure per webstack.md
- [x] Set up logging package (`internal/logging/`)
- [x] Create basic `cmd/server/main.go` entrypoint

## Phase 2: Container and LDAP Bootstrap

- [x] Finalize Dockerfile (verify build stages work)
- [x] Implement first-boot detection (empty volume check)
- [x] Initialize OpenLDAP `cn=config` via Go LDAP client
- [x] Apply `ldif/schema.ldif` on first boot
- [x] Create base DN and OUs (`ou=people`, `ou=groups`, `ou=serviceaccounts`)
- [x] Set LDAP ACLs
- [x] Generate SSH CA keypair (ed25519) and persist to `/data/ca/`
- [x] Provision initial admin user from `INITIAL_ADMIN_EMAIL`
- [x] Initialize SQLite schema
- [x] Read secrets from `RUNTIME_SECRETS` volume mount
- [x] Implement entrypoint.sh slapd startup and readiness check

## Phase 3: API Core

- [x] Set up chi router with TLS
- [x] Implement OAuth2/OIDC token validation middleware
- [x] Implement role-based authorization middleware (self, viewer, operator, admin, system)
- [x] Role lookup via LDAP group membership (authbox-admins, authbox-operators, authbox-viewers)
- [x] Implement standard error response envelope
- [x] Implement pagination (offset/limit) for list endpoints
- [x] `POST /api/v1/users` - create user in LDAP
- [x] `GET /api/v1/users` - list users (paginated)
- [x] `PUT /api/v1/users/{uid}` - update user
- [x] `POST /api/v1/users/{uid}/disable` - disable user (set nologin, remove groups, revoke FIDO2)
- [x] `POST /api/v1/users/{uid}/enable` - re-enable user (restore shell)
- [x] UID/GID uniqueness validation (409 on conflict)
- [x] UID/GID auto-assignment from configurable range
- [x] `POST /api/v1/groups` - create posixGroup or groupOfNames
- [x] `GET /api/v1/groups` - list groups (paginated)
- [x] `PUT /api/v1/groups/{cn}` - update group membership
- [x] `DELETE /api/v1/groups/{cn}` - remove group
- [x] `POST /api/v1/users/import` - bulk import (CSV and JSON)

## Phase 4: SSH Certificate Authority

- [x] `POST /api/v1/ssh/sign` - sign public key, return cert
- [x] Validate user exists and is active before signing
- [x] Set principals (username) and TTL from config (`SSH_CERT_TTL`)
- [x] `GET /api/v1/ssh/ca.pub` - return CA public key (unauthenticated)
- [x] `GET /api/v1/ssh/certs` - list issued certs (audit)

## Phase 5: FIDO2 Credential Management

- [x] `POST /api/v1/fido2/register` - accept `pamu2fcfg` output string, store for user
- [x] Validate credential string format before storing
- [x] `GET /api/v1/fido2/credentials/{uid}` - get credential mappings
- [x] `GET /api/v1/fido2/credentials` - all mappings in `pam_u2f` format (for Ansible sync)
- [x] Output format: `username:credential_id,public_key,es256,+presence`
- [x] Web UI: instructions to run `pamu2fcfg`, text input for paste, submit

## Phase 6: Service Accounts and Client Credentials

- [x] Service account CRUD in web UI
- [x] Generate client_id and client_secret at creation
- [x] `POST /oauth/token` - client credentials grant (issue bearer token)
- [x] Scope/role assignment per service account

## Phase 7: OIDC Authentication

- [x] OIDC discovery (fetch provider config from issuer URL)
- [x] Authorization code flow for web UI login
- [x] Token validation (verify signature, issuer, audience, expiry)
- [x] Session management (cookie-based for web UI)
- [x] Verify user exists in LDAP before granting access
- [x] Support Google and Entra ID (one active at a time)

## Phase 8: Web Frontend

- [x] Set up Go html/template rendering
- [x] Integrate HTMX for dynamic interactions
- [x] CSS framework with light/dark theme toggle
- [x] Login page (OIDC redirect)
- [x] Dashboard (stats, recent activity, system status)
- [x] User management (list, create, edit, disable, re-enable with confirmation)
- [x] Bulk import page (CSV and JSON upload with preview)
- [x] Group management (list, create, edit, delete with confirmation)
- [x] SSH cert issuance page (upload pubkey, download cert)
- [x] FIDO2 key enrollment page (instructions + paste input)
- [x] Service account management page
- [x] Log viewer (file list, paginated view, live-tail mode, level filter)
- [x] Status page (LDAP, replica sync, TLS expiry, backup status, disk usage)
- [x] Status indicator in header (green/warning/error)
- [x] Settings: OIDC provider configuration
- [x] Settings: session timeout (default 30 min)
- [x] Settings: UID/GID range
- [x] Settings: SSH CA info and cert TTL
- [x] Settings: LDAP configuration
- [x] Settings: log level and retention
- [x] Backup: export/import page
- [x] Backup: schedule daily slapcat (toggle, time, retention)
- [x] Destructive action confirmation pattern ("yesiagree" typed input)

## Phase 9: Backup and Recovery

- [x] `GET /api/v1/config/export` - export LDAP directory, cn=config, FIDO2 mappings, SQLite
- [x] `POST /api/v1/config/import` - import and rebuild from export
- [ ] CA key backup documentation

## Phase 10: Multi-Container HA

- [x] Read `ROLE` and `PRIMARY_HOST` env vars
- [x] Primary: serve all endpoints normally
- [x] Replica: reject writes (or proxy to primary), serve reads
- [x] `GET /internal/sync/state` - return current state version
- [x] `GET /internal/sync/changes?since={version}` - return deltas
- [x] `GET /internal/sync/snapshot` - return full SQLite state
- [x] Replica sync loop (snapshot on startup, poll for deltas)
- [x] Internal endpoint authentication (mTLS or shared secret)
- [x] Configure OpenLDAP syncrepl between primary and replica

## Phase 11: TLS Certificate Management

- [x] Load TLS cert/key from volume mount path
- [x] AWS credential mount for DNS-01 Let's Encrypt automation
- [x] Auto-renewal logic

## Phase 12: LDAP Configuration Management

- [x] Manage `cn=config` via LDAP protocol on localhost
- [ ] Web UI for LDAP settings (API wired, frontend uses Settings page)
- [x] Configure replication settings via API/UI

## Phase 13: Ansible Playbooks

- [ ] Verify enroll-host.yml works end-to-end
- [ ] Verify sync-fido2-mappings.yml works end-to-end
- [ ] Document required Ansible variables

## Phase 14: Testing

- [x] Unit tests for SSH cert signing
- [x] Unit tests for UID/GID assignment and conflict detection
- [x] Unit tests for role-based authorization
- [x] Unit tests for OIDC token validation
- [x] Unit tests for logging and rotation
- [x] Unit tests for backup round-trip, restore, cleanup
- [ ] Plan integration test suite (tests/integration/)

## Tech Debt

- [ ] Migrate existing hardcoded string literals to `internal/constants/constants.go` (shells, paths, time formats, route strings, defaults scattered across ldap/, web/api/, web/frontend/)

## SSH Cert Signing Page

- [x] HTMX inline signing (no page redirect)
- [x] Styled result box with principal, TTL, cert type
- [x] Copy to clipboard and download buttons
- [x] Cert recorded in SQLite audit log on sign
- [x] Issued Certificates table auto-refreshes after signing
- [x] Sortable table headers (user, serial, issued, expires) with asc/desc toggle
- [x] Button disabled during request (prevents double-click)
- [x] Daily background cleanup of expired cert records (90-day retention)
- [x] Rate limiting (10 certs per user per hour)
- [x] Visible counter showing remaining signs this hour

## Hardening: Drop Root Privileges

- [ ] Add `authbox` user in Dockerfile (`adduser -D -H authbox`)
- [ ] Install `su-exec` in Dockerfile (`apk add --no-cache su-exec`)
- [ ] In entrypoint.sh, chown `/data` and `/app/logs` to `authbox` before exec
- [ ] Change final exec to `exec su-exec authbox /usr/local/bin/authbox`
- [ ] Ensure secrets mount is readable by `authbox` (group read via `chgrp`)
- [ ] Verify LDAP client operations still work as unprivileged user
- [ ] Verify TLS cert renewal still works (writes to /data/tls/)

## Reusable Table Component

- [ ] Create `tableRenderer` helper (columns, sort, pagination, HTMX partial URL)
- [ ] Column definition struct (key, label, sortable flag)
- [ ] Sortable header rendering with asc/desc indicators
- [ ] Pagination footer (Prev/Next buttons, "Showing X-Y of Z")
- [ ] Row render callback function per table
- [ ] Migrate SSH certs partial to use tableRenderer
- [ ] Migrate users list partial to use tableRenderer
- [ ] Migrate groups list partial to use tableRenderer
- [ ] Migrate service accounts partial to use tableRenderer
- [ ] Migrate FIDO2 keys partial to use tableRenderer
