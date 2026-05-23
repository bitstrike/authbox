# Authbox - Implementation Checklist

## Phase 1: Project Scaffolding

- [ ] Initialize Go module (`go mod init`)
- [ ] Create project directory structure per webstack.md
- [ ] Set up logging package (`internal/logging/`)
- [ ] Create basic `cmd/server/main.go` entrypoint

## Phase 2: Container and LDAP Bootstrap

- [ ] Finalize Dockerfile (verify build stages work)
- [ ] Implement first-boot detection (empty volume check)
- [ ] Initialize OpenLDAP `cn=config` via Go LDAP client
- [ ] Apply `ldif/schema.ldif` on first boot
- [ ] Create base DN and OUs (`ou=people`, `ou=groups`, `ou=serviceaccounts`)
- [ ] Set LDAP ACLs
- [ ] Generate SSH CA keypair (ed25519) and persist to `/data/ca/`
- [ ] Provision initial admin user from `INITIAL_ADMIN_EMAIL`
- [ ] Initialize SQLite schema
- [ ] Read secrets from `RUNTIME_SECRETS` volume mount
- [ ] Implement entrypoint.sh slapd startup and readiness check

## Phase 3: API Core

- [ ] Set up chi router with TLS
- [ ] Implement OAuth2/OIDC token validation middleware
- [ ] Implement role-based authorization middleware (self, viewer, operator, admin, system)
- [ ] Role lookup via LDAP group membership (authbox-admins, authbox-operators, authbox-viewers)
- [ ] Implement standard error response envelope
- [ ] Implement pagination (offset/limit) for list endpoints
- [ ] `POST /api/v1/users` - create user in LDAP
- [ ] `GET /api/v1/users` - list users (paginated)
- [ ] `PUT /api/v1/users/{uid}` - update user
- [ ] `POST /api/v1/users/{uid}/disable` - disable user (set nologin, remove groups, revoke FIDO2)
- [ ] `POST /api/v1/users/{uid}/enable` - re-enable user (restore shell)
- [ ] UID/GID uniqueness validation (409 on conflict)
- [ ] UID/GID auto-assignment from configurable range
- [ ] `POST /api/v1/groups` - create posixGroup or groupOfNames
- [ ] `GET /api/v1/groups` - list groups (paginated)
- [ ] `PUT /api/v1/groups/{cn}` - update group membership
- [ ] `DELETE /api/v1/groups/{cn}` - remove group
- [ ] `POST /api/v1/users/import` - bulk import (CSV and JSON)

## Phase 4: SSH Certificate Authority

- [ ] `POST /api/v1/ssh/sign` - sign public key, return cert
- [ ] Validate user exists and is active before signing
- [ ] Set principals (username) and TTL from config (`SSH_CERT_TTL`)
- [ ] `GET /api/v1/ssh/ca.pub` - return CA public key (unauthenticated)
- [ ] `GET /api/v1/ssh/certs` - list issued certs (audit)

## Phase 5: FIDO2 Credential Management

- [ ] `POST /api/v1/fido2/register` - accept `pamu2fcfg` output string, store for user
- [ ] Validate credential string format before storing
- [ ] `GET /api/v1/fido2/credentials/{uid}` - get credential mappings
- [ ] `GET /api/v1/fido2/credentials` - all mappings in `pam_u2f` format (for Ansible sync)
- [ ] Output format: `username:credential_id,public_key,es256,+presence`
- [ ] Web UI: instructions to run `pamu2fcfg`, text input for paste, submit

## Phase 6: Service Accounts and Client Credentials

- [ ] Service account CRUD in web UI
- [ ] Generate client_id and client_secret at creation
- [ ] `POST /oauth/token` - client credentials grant (issue bearer token)
- [ ] Scope/role assignment per service account

## Phase 7: OIDC Authentication

- [ ] OIDC discovery (fetch provider config from issuer URL)
- [ ] Authorization code flow for web UI login
- [ ] Token validation (verify signature, issuer, audience, expiry)
- [ ] Session management (cookie-based for web UI)
- [ ] Verify user exists in LDAP before granting access
- [ ] Support Google and Entra ID (one active at a time)

## Phase 8: Web Frontend

- [ ] Set up Go html/template rendering
- [ ] Integrate HTMX for dynamic interactions
- [ ] CSS framework with light/dark theme toggle
- [ ] Login page (OIDC redirect)
- [ ] Dashboard (stats, recent activity, system status)
- [ ] User management (list, create, edit, disable, re-enable with confirmation)
- [ ] Bulk import page (CSV and JSON upload with preview)
- [ ] Group management (list, create, edit, delete with confirmation)
- [ ] SSH cert issuance page (upload pubkey, download cert)
- [ ] FIDO2 key enrollment page (instructions + paste input)
- [ ] Service account management page
- [ ] Log viewer (file list, paginated view, live-tail mode, level filter)
- [ ] Status page (LDAP, replica sync, TLS expiry, backup status, disk usage)
- [ ] Status indicator in header (green/warning/error)
- [ ] Settings: OIDC provider configuration
- [ ] Settings: session timeout (default 30 min)
- [ ] Settings: UID/GID range
- [ ] Settings: SSH CA info and cert TTL
- [ ] Settings: LDAP configuration
- [ ] Settings: log level and retention
- [ ] Backup: export/import page
- [ ] Backup: schedule daily slapcat (toggle, time, retention)
- [ ] Destructive action confirmation pattern ("yesiagree" typed input)

## Phase 9: Backup and Recovery

- [ ] `GET /api/v1/config/export` - export LDAP directory, cn=config, FIDO2 mappings, SQLite
- [ ] `POST /api/v1/config/import` - import and rebuild from export
- [ ] CA key backup documentation

## Phase 10: Multi-Container HA

- [ ] Read `ROLE` and `PRIMARY_HOST` env vars
- [ ] Primary: serve all endpoints normally
- [ ] Replica: reject writes (or proxy to primary), serve reads
- [ ] `GET /internal/sync/state` - return current state version
- [ ] `GET /internal/sync/changes?since={version}` - return deltas
- [ ] `GET /internal/sync/snapshot` - return full SQLite state
- [ ] Replica sync loop (snapshot on startup, poll for deltas)
- [ ] Internal endpoint authentication (mTLS or shared secret)
- [ ] Configure OpenLDAP syncrepl between primary and replica

## Phase 11: TLS Certificate Management

- [ ] Load TLS cert/key from volume mount path
- [ ] AWS credential mount for DNS-01 Let's Encrypt automation
- [ ] Auto-renewal logic

## Phase 12: LDAP Configuration Management

- [ ] Manage `cn=config` via LDAP protocol on localhost
- [ ] Web UI for LDAP settings
- [ ] Configure replication settings via API/UI

## Phase 13: Ansible Playbooks

- [ ] Verify enroll-host.yml works end-to-end
- [ ] Verify sync-fido2-mappings.yml works end-to-end
- [ ] Document required Ansible variables

## Phase 14: Testing

- [ ] Unit tests for SSH cert signing
- [ ] Unit tests for UID/GID assignment and conflict detection
- [ ] Unit tests for role-based authorization
- [ ] Unit tests for OIDC token validation
- [ ] Unit tests for logging and rotation
- [ ] Plan integration test suite (tests/integration/)
