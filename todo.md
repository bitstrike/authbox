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

- [x] Create `tableRenderer` helper (columns, sort, pagination, HTMX partial URL)
- [x] Column definition struct (key, label, sortable flag)
- [x] Sortable header rendering with asc/desc indicators
- [x] Pagination footer (Prev/Next buttons, "Showing X-Y of Z")
- [x] Page size selector (10 / 25 / 50 / 100)
- [x] Row render callback function per table
- [x] Optional filterable flag with search input (magnifying glass icon, debounced server-side filter across all columns)
- [x] Styled empty state ("No results" message when zero rows)
- [x] Standardized row actions column (Edit/Delete/View links)
- [x] Loading indicator (spinner/skeleton via hx-indicator)
- [ ] Export/download button (CSV of current filtered/sorted view)
- [x] Row count badge near table title ("Showing X-Y of Z")
- [x] Migrate SSH certs partial to use tableRenderer
- [x] Migrate users list partial to use tableRenderer
- [x] Migrate groups list partial to use tableRenderer
- [x] Migrate service accounts partial to use tableRenderer
- [x] Migrate FIDO2 keys partial to use tableRenderer

## Dashboard Page

- [x] Stat cards show real counts:
  - Total Users
  - Active Users
  - Disabled Users
  - Groups (total, with "X posix / Y role" subtitle)
  - Certs Active Now (unexpired)
  - FIDO2 Keys (total registered)
  - Warnings Today (count of WARN in today's log, yellow-tinted when > 0)
  - Errors Today (count of ERROR in today's log, red-tinted when > 0)
- [x] Dashboard handler queries LDAP/SQLite/logs for stats and passes to template
- [x] System status panel inline on dashboard (LDAP connection, TLS expiry, replication state)
- [x] Status panel auto-refreshes every 30s via HTMX
- [x] Recent activity table uses reusable table component (sortable, filterable, paginated)
- [x] Green dot in nav still links to /status for detailed view
- [ ] Future: customizable widget selection with drag-to-reorder (localStorage)

## DNS Provider Abstraction (TLS/ACME)

- [ ] Replace custom ACME/Route53 code with `github.com/go-acme/lego/v4`
- [ ] Delete `internal/tls/route53.go` (custom SigV4, XML building)
- [ ] Rewrite `obtainCert()` in `manager.go` to use lego client
- [ ] Lego auto-detects provider from env vars (AWS_ACCESS_KEY_ID for Route53, CLOUDFLARE_DNS_API_TOKEN for Cloudflare, etc.)
- [ ] Pass secrets as env vars to lego (read from secrets files, export before calling lego)
- [ ] Verify Route53 still works after conversion
- [ ] Test Cloudflare provider
- [ ] Document supported DNS providers in README

## User Enable/Disable Toggle in Edit Form

- [x] Add status indicator to `user_form.html` (visible only in edit mode, use `.Content.User.Disabled`)
- [x] Add toggle button: POST to `/users/{uid}/disable` or `/users/{uid}/enable`
- [x] Include `confirm=yesiagree` hidden field with `hx-confirm` prompt on both actions
- [x] Disable confirmation message must warn: "This will set shell to /sbin/nologin and revoke FIDO2 credentials"
- [x] Conditionally show Disable (operators+) or Enable (admin only) using `.IsAdmin`/`.IsOperator` from PageData
- [x] No handler/struct changes needed (`User.Disabled` and role bools already available in template)
- [ ] Known limitation: enable hardcodes `/bin/bash` instead of restoring previous shell (separate fix)

## Dark Mode Table Header Contrast Fix

- [x] Table header sort links now use `text-blue-600` base class (dark: `#60a5fa`)
- [x] Added `.hover\:text-blue-600:hover` rule to style.css (light: `#2563eb`)
- [x] Added `.dark .hover\:text-blue-600:hover` rule (dark: `#93bbfd`, blue-300)
- [x] Consistent with existing `.dark .text-blue-600 { color: #60a5fa }` pattern

## Code Documentation

- [x] Block comments added to all Go source files (35 files across cmd/ and internal/)

## Fix: Backup Export Button (session auth, not bearer token)

- [x] Add frontend handler `actionExportBackup` that calls `backup.CreateExport()` directly
- [x] Register `GET /backup/export` in admin route group (session-protected)
- [x] Change `backup.html` link from `/api/v1/config/export` to `/backup/export`
- [x] API endpoint remains for service account/automation use (bearer token)

## Fix: Backup Import Button (404 - route not registered)

- [x] Add frontend handler `actionImportBackup` in handlers.go
- [x] Read multipart form: archive file + confirm field
- [x] Validate `confirm == "yesiagree"`, reject otherwise
- [x] Call `backup.ImportExport()` to parse the uploaded archive
- [x] Call `backup.RestoreLDAP()` for directory and cn=config LDIFs
- [x] Call `backup.RestoreState()` for SQLite application state
- [x] Register `POST /backup/import` in admin route group (session-protected)
- [x] Redirect to `/backup` with success or re-render with error message
- [x] API endpoint at `POST /api/v1/config/import` remains for automation (bearer token)

## LDAP Restore via Staged Files (live-restore pattern)

- [x] Create `/data/live-restore/` directory convention for staged restore files
- [x] `actionImportBackup`: write extracted LDIFs to `/data/live-restore/` instead of calling slapadd directly
- [x] `actionImportBackup`: restore SQLite state immediately (independent of slapd)
- [x] `actionImportBackup`: trigger container restart after staging (os.Exit, rely on Docker restart policy)
- [x] `entrypoint.sh`: on startup, check if `/data/live-restore/` exists
- [x] `entrypoint.sh`: if restore dir found, wipe MDB (`/var/lib/openldap/*`) and cn=config (`/etc/openldap/slapd.d/*`)
- [x] `entrypoint.sh`: run `slapadd -l /data/live-restore/directory.ldif`
- [x] `entrypoint.sh`: run `slapadd -b cn=config -l /data/live-restore/config.ldif` (if file exists)
- [x] `entrypoint.sh`: remove `/data/live-restore/` after successful restore
- [x] `entrypoint.sh`: log errors and start slapd with empty DB if restore fails (admin can retry)
- [x] CLI escape hatch: admin can manually place LDIF in `/data/live-restore/` and restart container
- [x] Document the restore workflow in README

## Pre-Import Safety Backup in Entrypoint

- [x] Before wiping MDB, run `slapcat` to `/data/backups/pre-import-backup-YYYYMMDD-HHMMSS-directory.ldif`
- [x] Also run `slapcat -b cn=config` to `/data/backups/pre-import-backup-YYYYMMDD-HHMMSS-config.ldif`
- [x] If slapcat fails: abort import, log "pre-import backup failed (disk full?)", start slapd with existing data unchanged
- [x] If slapcat succeeds: proceed with wipe and slapadd
- [x] If slapadd fails: attempt rollback using the pre-import backup LDIFs
- [x] If rollback also fails: log critical error directing operator to investigate disk/permissions
- [x] On successful import: keep pre-import backup in `/data/backups/` as rollback point
- [x] On failed import with successful rollback: rename staged files to `.failed` to prevent retry loop

## API Import Endpoint: Switch to Staged Pattern

- [x] Update `POST /api/v1/config/import` to use the same live-restore staging as the frontend
- [x] Validate archive and `X-Confirm: yesiagree` header (already done)
- [x] Stage LDIFs to `/data/live-restore/` instead of calling slapadd directly
- [x] Restore SQLite state immediately
- [x] Return `200 { "message": "restore staged, container restarting", "restart": true }`
- [x] Exit process after short delay (same as frontend handler)

## Add Logger to Frontend Deps

- [x] Add `Log *logging.Logger` field to `frontend.Deps` struct
- [x] Pass logger from `main.go` when constructing Deps
- [x] Log import events: "backup import started" (with user email), "staging LDIF files", "sqlite state restored, triggering restart"
- [x] Log disable/enable user events (who disabled/enabled whom)
- [x] Call `file.Sync()` before `os.Exit` in import handler to ensure log flush
- [x] Entrypoint already logs restore steps via echo (no change needed there)

## employeeType Attribute Support

- [ ] Add `EmployeeType string` field to `ldap.User` struct
- [ ] Include `"employeeType"` in LDAP search attribute lists (GetUser, ListUsers)
- [ ] Read `employeeType` in `entryToUser`
- [ ] Write `employeeType` in `CreateUser` and `UpdateUser` (if non-empty)
- [ ] Add `<select>` dropdown to `user_form.html` with values: (empty), employee, contractor, contact
- [ ] Display colored dot or short badge in user list partial (blue=employee, orange=contractor, gray=contact)
- [ ] Support `employeeType` in bulk CSV/JSON import
- [ ] Add employeeType filter option to user list (like existing status filter)
- [ ] Dashboard: add "Contacts" card showing count of inetOrgPerson entries without posixAccount (only shown if contacts exist)
- [ ] Update project.md with employeeType documentation
- [ ] Update README if needed

## Settings Page Redesign (sidebar navigation)

- [x] Refactor `settings.html` to two-column layout: left nav + right content panel
- [x] Left nav lists settings categories (OIDC Provider, Session, UID/GID Range, SSH CA, LDAP, Logging, Employee Types)
- [x] Right panel loads selected category content via HTMX partial swap
- [x] Each settings category becomes its own partial template
- [x] Default selection on page load (OIDC via hx-trigger="load")
- [x] Active nav item highlighted
- [ ] Mobile/narrow: collapse to stacked layout or hamburger
- [x] Register partial routes: `/settings/oidc`, `/settings/session`, `/settings/uid-range`, `/settings/ssh-ca`, `/settings/ldap`, `/settings/logging`, `/settings/employee-types`
- [x] Migrate existing settings sections into individual partials

## Reusable Sidebar Layout Component

- [x] Create `SidebarRenderer` (similar pattern to `TableRenderer`)
- [x] Define `SidebarConfig` struct: PanelID, NavItems (label + URL), DefaultURL
- [x] Render two-column layout: left nav with HTMX links, right panel with load trigger
- [x] Include JS for active nav item highlighting (generic, class-based)
- [x] Rename CSS class from `settings-nav-item` to `sidebar-nav-item`
- [x] Refactor settings page to use `SidebarRenderer`
- [x] Refactor backup page to use `SidebarRenderer` (Export, Import, Schedule, CA Key sections)
- [x] Register backup partials: `/backup/export-panel`, `/backup/import-panel`, `/backup/schedule`, `/backup/ca-key`
