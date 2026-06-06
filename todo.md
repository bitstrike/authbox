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
- [x] Install `tzdata` in Dockerfile for timezone support
- [x] Add `TZ` env var to docker-compose (defaults to UTC, set IANA timezone to override)

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
- [x] First name and Last Name of user will be updated from jwt information on every login

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

### New Unit Tests (recent changes)

#### Flash package (`internal/flash/`)
- [x] `Set` writes cookie with correct format (type|message, URL-encoded)
- [x] `Get` reads and clears the cookie (returns message, cookie deleted after read)
- [x] `Get` returns nil when no cookie present
- [x] `Get` handles malformed cookie values gracefully (no panic)

#### TruncateForRestore (`internal/db/`)
- [x] Truncates fido2_credentials, service_accounts, ssh_certs tables
- [x] Does not affect employee_types table
- [x] RestoreState succeeds on duplicate data after truncation (round-trip test)

#### CSV import validation (`internal/web/frontend/`)
- [ ] Contact-type rows: UID/GID forced to 0, shell forced to `/sbin/nologin`
- [ ] Non-contact rows: UID/GID outside configured range rejected
- [ ] Mixed valid/invalid rows: entire import aborted on any validation failure
- [ ] Empty phone fields produce no LDAP attributes

#### CreateUser conditional objectClass (`internal/ldap/`)
- [ ] UID/GID > 0: request includes posixAccount objectClass and posix attributes
- [ ] UID/GID == 0: request uses inetOrgPerson only, skips posix attributes
- [ ] Phone attributes included only when non-empty

#### UpdateUser conditional posix attributes (`internal/ldap/`)
- [ ] UID/GID > 0: modify request includes posix attribute replacements
- [ ] UID/GID == 0: modify request skips posix attributes
- [ ] Phone attributes: non-empty values replaced, empty values cleared

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

- [x] Add `EmployeeType string` field to `ldap.User` struct
- [x] Include `"employeeType"` in LDAP search attribute lists (GetUser, ListUsers)
- [x] Read `employeeType` in `entryToUser`
- [x] Write `employeeType` in `CreateUser` and `UpdateUser` (if non-empty)
- [x] Add `<select>` dropdown to `user_form.html` populated dynamically from DB
- [x] Display colored dot or short badge in user list partial (emoji from DB)
- [x] Support `employeeType` in bulk CSV/JSON import
- [x] Add employeeType filter option to user list (like existing status filter)
- [x] Dashboard: add "Contacts" card showing count of inetOrgPerson entries without posixAccount (only shown if contacts exist)
- [x] Change `ListUsers` LDAP filter from `(objectClass=posixAccount)` to `(objectClass=inetOrgPerson)` so contacts appear in user list
- [x] Update project.md with employeeType documentation
- [ ] Update README if needed

### Employee Types SQLite Storage

- [x] Create `employee_types` table: id, value (UNIQUE), label, emoji, sort_order
- [x] Migration seeds 4 defaults: contractor =👷, employee = 👤, service = 🤖, contact = 🪪
- [x] Use `INSERT OR IGNORE` so seeding is idempotent (skipped if types already exist)
- [x] Repository methods: ListEmployeeTypes, CreateEmployeeType, DeleteEmployeeType, UpdateEmployeeType
- [x] Add `EmployeeTypes []db.EmployeeType` to `ExportData` struct
- [x] Query employee_types in `exportAppState`
- [x] Restore employee_types in `RestoreState` (INSERT OR REPLACE to merge with defaults)

### Employee Types Settings UI

- [x] Implement `settings_employee_types.html` partial (replace placeholder)
- [x] Display table of current types: emoji, value, label, sort order, remove button
- [x] Add form: emoji input, value input, label input, Add button
- [x] HTMX inline add/remove (no full page reload)
- [x] Register POST route for add/delete actions
- [x] User form dropdown queries ListEmployeeTypes to populate options

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

## User Create/Edit Form Fixes

### Create User
- [x] UID/GID must always be within the UID/GID range specified in settings (validate on submit)
- [x] Pre-fill UID/GID fields with next available UID/GID for convenience
- [x] Pre-fill home directory using uid typed in the uid text field (auto-fill via JS as user types)
- [x] Employee type defaults to "employee" on create form

### Edit User
- [x] UID/GID fields auto-fill with current UID/GID of user being edited (already works)
- [x] If employeeType is "contact", hide posixAccount fields (UID/GID, home directory, login shell)
- [x] If employeeType is changed away from "contact", show posixAccount fields again
- [x] Use JS to toggle field visibility based on employeeType dropdown selection

## Fix: UID/GID Uniqueness Validation in Frontend Handlers

- [x] `actionCreateUser`: validate UID uniqueness via `UIDExists()` before calling `CreateUser`
- [x] `actionCreateUser`: validate GID uniqueness via `GIDExists()` before calling `CreateUser`
- [x] `actionUpdateUser`: validate UID uniqueness if changed (compare to existing user's UID)
- [x] `actionUpdateUser`: validate GID uniqueness if changed
- [x] On conflict, re-render form with error message (same pattern as API's 409 response)
- [x] Pre-fill should also check GID against groups (not just users) to avoid UID=GID collision with existing posixGroup
- [x] Consider race condition: if two admins submit simultaneously, LDAP itself should reject the duplicate (belt and suspenders)

## Fix: Service Account Secret Hash Mismatch

- [x] Frontend `actionCreateServiceAccount` uses SHA256 (`hashSecret`) to hash the client_secret
- [x] Token endpoint `tokenHandler` uses bcrypt (`bcrypt.CompareHashAndPassword`) to verify
- [x] These are incompatible - service accounts created via web UI can never authenticate
- [x] Fix: change `hashSecret()` in `actions.go` to use `bcrypt.GenerateFromPassword`
- [x] Remove the SHA256 `hashSecret` function (dead code after fix)
- [x] Verify API `createServiceAccount` in `serviceaccounts.go` already uses bcrypt (it does)
- [x] Any existing service accounts created via web UI will need to be recreated after fix

## Fix: HTMX Delete Buttons Hit API (401 - no bearer token)

Frontend HTMX buttons call API endpoints that require bearer tokens. Browser only sends session cookie.

### Service Account Delete
- [x] Add frontend handler for service account deletion (session-authenticated)
- [x] Register `POST /service-accounts/{clientID}/delete` in admin route group
- [x] Handler calls `repo.DeleteServiceAccount(clientID)` and returns refreshed list partial
- [x] Update partial to use frontend route instead of `/api/v1/service-accounts`

### FIDO2 Credential Revoke
- [x] Add frontend handler for FIDO2 credential deletion (session-authenticated)
- [x] Register `POST /fido2/credentials/{id}/revoke` in appropriate route group
- [x] Handler calls `repo.DeleteFIDO2CredentialByID(id)` and returns refreshed list partial
- [x] Update partial to use frontend route instead of `/api/v1/fido2/credentials/{id}`

## Fix: Service Account Tokens Not Accepted by API Middleware

- [x] `TokenMiddleware` only validates OIDC tokens via `verifier.Verify()`
- [x] When OIDC verification fails, it returns 401 with no fallback
- [x] `ValidateServiceToken` in `token.go` exists but is never called by the middleware
- [x] Service accounts can obtain tokens (`POST /oauth/token` works) but can't use them on any endpoint
- [x] Fix: add fallback in `TokenMiddleware` - if OIDC verify fails, try `api.ValidateServiceToken(token)`
- [x] If service token is valid, set claims (clientID as email/sub) and role from the token entry
- [x] This requires `TokenMiddleware` to accept a service token validator function (avoid circular import between auth and api packages)
- [x] Consider: pass a `func(string) (string, string, bool)` validator to `TokenMiddleware` as a parameter

## Delete User (admin, disabled accounts only)

- [x] Add `DeleteUser(uid string)` method to LDAP client (ldap.Del request)
- [x] Add delete button to user edit form (visible only when user is disabled AND role is admin)
- [x] Require "yesiagree" confirmation with warning text explaining UID reuse risk
- [x] Warning: "This removes the user from the directory. The UID/GID will become available for reassignment. If this user owned files on any host, a future user with the same UID will inherit those files. Disable instead if unsure."
- [x] Register `POST /users/{uid}/delete` in admin route group
- [x] Handler validates confirm field, calls LDAP DeleteUser, redirects to /users
- [x] Only allow deletion of disabled accounts (reject if user is not disabled)
- [x] Add `DELETE /api/v1/users/{uid}` API endpoint (admin role, same validation)
- [x] Log deletion event (who deleted whom)
- [x] Fix page layout that was corrupted when adding the "Delete" button
- [x] Delete button: stack confirm input on its own line (label above, full-width input like backup import)
- [x] Delete button: move button to its own line below the input (not inline flex)
- [x] Delete button: change class from `btn btn-secondary text-sm text-red-600` to `btn btn-danger` (matches Import button)
- [x] Delete button: remove `flex items-end gap-2` wrapper, use `mb-4` spacing between input and button
- [x] Add Flash notification when delete user

## Flash Notification System (Flash package)

Server-side flash messages rendered as a colored top-bar notification (AWS console style).

### Infrastructure
- [x] Add `Flash` struct (Type: success/error/warning, Message string) to session data
- [x] Add `setFlash(w, r, type, message)` helper that writes flash to session
- [x] Add `getFlash(w, r)` helper that reads and clears flash from session (one-time read)
- [x] Add `Flash` field to base `PageData` struct
- [x] Populate `Flash` in the base page data builder (call `getFlash` on every page render)

### Template and CSS
- [x] Add notification markup to base layout (renders above `<main>` content when Flash is set)
- [x] CSS: `.notification` base class (full-width bar, padding, flex with text + close button)
- [x] CSS: `.notification-success` (green background, white text)
- [x] CSS: `.notification-error` (red background, white text)
- [x] CSS: `.notification-warning` (yellow/orange background, dark text)
- [x] CSS: dark mode variants for all three types
- [x] Close button (X) to dismiss manually
- [x] JS: auto-dismiss after 5 seconds (fade-out animation, then remove from DOM)

### Adopt in Handlers
- [x] Delete user: flash success "User {uid} deleted" before redirect to /users
- [x] Enable user: flash success "User {uid} enabled"
- [x] Disable user: flash success "User {uid} disabled"
- [x] Create user: flash success "User {uid} created"
- [ ] Backup import: flash success "Restore staged, container restarting" (deferred - container exits immediately, current inline message is sufficient)
- [x] Backup import: flash error on invalid confirmation text, redirect to /backup
- [x] Backup import: all error paths use flash + redirect instead of inline HTML error- [x] Service account delete: flash success "Service account deleted" (via HX-Trigger)
- [x] FIDO2 credential revoke: flash success "FIDO2 credential revoked" (via HX-Trigger)
- [x] Group create/update/delete: flash success messages
- [x] Bulk import: flash success "{n} users imported"
- [x] FIDO2 register: flash success "FIDO2 credential registered for {uid}"
- [x] Add member to group: flash success "Member {uid} added to {cn}" (redirect-based)
- [x] Create employee type: flash via HX-Trigger "Employee type added"
- [x] Delete employee type: flash via HX-Trigger "Employee type removed"

### Backup Export Flash (file download)
- [x] Change export button to use JS `fetch()` instead of plain link/form
- [x] On success: trigger browser download via blob URL, then inject client-side flash "Export complete"
- [x] On error: inject client-side flash error with message from response

## Fix: Backup Import UNIQUE Constraint Failure

Importing an archive exported from the same instance fails because `RestoreState` inserts rows that already exist.

- [x] In `RestoreState`, truncate tables before inserting: `DELETE FROM service_accounts`, `DELETE FROM fido2_credentials`, `DELETE FROM ssh_certs`
- [x] Employee types already use upsert (`UpsertEmployeeType`) so no change needed there
- [ ] Verify round-trip: export, then import on same instance without error
- [x] Backup > Export should flash error/success

## Fix: CSV Import - Contact Type and UID/GID Validation

### CreateUser: support inetOrgPerson-only entries (no posixAccount)
- [x] If UIDNumber == 0 and GIDNumber == 0, use objectClass `["top", "inetOrgPerson"]` (skip posixAccount)
- [x] Skip uidNumber, gidNumber, homeDirectory, loginShell attributes when not posixAccount
- [x] This allows contacts to be created without posix fields

### UpdateUser: skip posixAccount attributes for contacts
- [x] If UIDNumber == 0 and GIDNumber == 0, skip Replace for uidNumber, gidNumber, homeDirectory, loginShell
- [x] Prevents "Object Class Violation: attribute 'uidNumber' not allowed" on inetOrgPerson-only entries

### User list: hide UID/GID for contacts
- [x] In user list partial, display `-` for UID/GID columns when employeeType is "contact"
- [x] Matches edit form behavior (posix fields hidden for contacts)

### Contact disable/delete logic
- [x] Skip `loginShell` modification when disabling a contact (no posixAccount, would cause Object Class Violation)
- [x] Allow deletion of contacts without requiring disable first (contacts have no login capability)
- [x] Hide Disable button in user edit form when employeeType is "contact"
- [x] Set `Disabled` to true for contacts in `entryToUser` (or use a separate check) so delete path works

### Import: UID/GID range validation
- [x] Read configured UID/GID range (from config) at start of import
- [x] For non-contact rows: validate UID/GID are within configured range
- [x] For contact rows: allow empty UID/GID (skip range check)
- [x] If any row fails validation, abort entire import (no partial imports)
- [x] Collect all validation errors and flash them (e.g., "Row 3: UID 500 outside range 10000-60000")

### Import: contact-type handling
- [x] If employeeType is "contact" and UID/GID are empty, set UIDNumber=0 and GIDNumber=0
- [x] If employeeType is "contact", force loginShell to `/sbin/nologin` regardless of CSV value
- [x] If employeeType is "contact", allow empty homeDirectory



## Phone Number Attributes (inetOrgPerson)

Add support for standard LDAP phone attributes: telephoneNumber (work), mobile (cell), homePhone, facsimileTelephoneNumber (fax), pager.

### User Struct and LDAP Operations
- [x] Add fields to `User` struct: `TelephoneNumber`, `Mobile`, `HomePhone`, `Fax`, `Pager` (all `string`)
- [x] Add attributes to LDAP search lists in `GetUser` and `ListUsers`
- [x] Read attributes in `entryToUser`
- [x] Write attributes in `CreateUser` (if non-empty)
- [x] Write attributes in `UpdateUser` (if non-empty, replace; if empty, delete attribute)

### User Form
- [x] Add "Phone Numbers" field group to `user_form.html` (below email, above employeeType)
- [x] Fields: Work Phone, Mobile, Home Phone, Fax, Pager (all optional text inputs)
- [x] Phone fields remain visible for all employee types (including contacts)

### Frontend Handler
- [x] Read phone form values in `actionCreateUser` and `actionUpdateUser`
- [x] Map form values to User struct fields before calling LDAP

### Bulk Import
- [x] Add phone columns to CSV import (telephoneNumber, mobile, homePhone, fax, pager)
- [x] Add phone fields to JSON import schema
- [x] Document new columns in import instructions/help text
- [x] Empty phone fields are valid (optional attributes, skip writing to LDAP if blank)
- [x] Update `samples/csv/users.csv` with phone number columns

## Bulk Operations (TableRenderer checkbox selection)

Add row selection checkboxes to the reusable TableRenderer with a bulk action bar.

### TableRenderer Changes
- [x] Add `Selectable bool` option to table config
- [x] When enabled, render checkbox column as first column
- [x] Header checkbox toggles all visible rows (select all / deselect all)
- [x] JS: track selected row IDs in a Set, update count badge
- [x] "X selected" indicator appears when at least one row is checked
- [x] Bulk action bar appears above table when selections exist (hidden otherwise)
- [x] Each table page defines available bulk actions via config
- [x] Selected IDs submitted as JSON array to bulk action endpoint
- [x] Destructive bulk actions require "yesiagree" confirmation

### User List Bulk Actions
- [x] Bulk disable (set nologin, revoke FIDO2 for all selected)
- [x] Bulk delete (only allowed for disabled accounts)
- [x] Bulk change employeeType
- [x] Bulk add to group
- [x] Bulk remove from group
- [x] Bulk export selected as CSV

### Group List Bulk Actions
- [x] Bulk delete groups

### SSH Certs Bulk Actions
- [x] Bulk delete expired/selected certs

### FIDO2 Bulk Actions
- [x] Bulk revoke selected credentials

### Service Accounts Bulk Actions
- [x] Bulk delete selected accounts

## HTTP to HTTPS Redirect

- [ ] Listen on HTTP port (80 or configurable) in addition to HTTPS
- [ ] HTTP listener redirects all requests to HTTPS (301 permanent redirect)

## Bulk Action UX: Smart Confirmation with State Awareness

Bulk operations (especially delete) silently skip ineligible items and report after the fact.
Admin loses selection context and must re-find skipped users manually. Fix with inline
eligibility feedback: highlight conflicting rows and show count before the action fires.

### Problem

- Admin selects 20 users, clicks "Delete"
- 6 are still active, get skipped silently
- Response: "14 deleted (6 skipped - not disabled)"
- Admin must now find those 6 again, bulk disable, then re-select and delete
- Same issue with "Disable" action: contacts are silently skipped (cannot be disabled)

### Proposed Solution: Two-Phase Inline Confirmation with Row Highlighting

Instead of a modal dialog, use the existing bulk action bar as the confirmation UI.
When an action has eligibility rules, the first click evaluates and highlights conflicts;
the second click confirms. Ineligible rows get a `conflict-row-bg` background so the
admin can visually identify them without reading a list of UIDs.

### Implementation

#### 1. Add `EligibleIf` field to `BulkAction` struct (table.go)
- [x] New optional field: `EligibleIf string` (JS expression evaluated per row)
- [x] Rendered as `data-eligible-if` attribute on the bulk action button
- [x] If empty, all selected rows are eligible (backward compatible, no two-phase)
- [x] Expression references `data-*` attributes on the row checkbox element

#### 2. Add data attributes to user table row checkboxes
- [x] Add `data-disabled="true|false"` to each user row checkbox
- [x] Add `data-type="{employeeType}"` to each user row checkbox
- [x] Other tables only need attributes if they define `EligibleIf` rules (opt-in)

#### 3. Extend `submitBulk()` JS in table.go to two-phase confirm
- [x] First click: if `data-eligible-if` is set and button not yet confirmed:
  - Evaluate eligibility expression against each selected row's data attributes
  - Add `conflict-row-bg` class to ineligible rows
  - Update bulk bar count: "25 selected (5 ineligible)"
  - Change button text to include eligible count: "Delete (20)"
  - Mark button as `bulk-confirmed` (CSS class), return without firing
- [x] Second click: button has `bulk-confirmed`, proceed with existing fetch logic
- [x] Cancel/deselect: removing selections clears `conflict-row-bg` and resets button state
- [x] If zero ineligible on first click, skip straight to confirm (no extra click needed)

#### 4. Add `evaluateEligibility(checkbox, expression)` JS helper
- [x] Reads `data-*` attributes from the checkbox element
- [x] Evaluates the expression string against those values
- [x] Returns boolean (true = eligible, false = conflict)

#### 5. CSS: `conflict-row-bg` class (style.css)
- [x] Light mode: soft amber/orange background (`#fffbeb`)
- [x] Dark mode: muted amber tint (`rgba(180, 83, 9, 0.15)`)
- [x] Must be visually distinct from selected-row highlight (blue tint)
- [x] Both classes can coexist on the same row (selected AND conflicting)

#### 6. Define eligibility rules for user bulk actions

| Action | EligibleIf expression | Conflict means |
|--------|----------------------|----------------|
| Delete | `disabled=='true' \|\| type=='contact'` | Active non-contact user |
| Disable | `type!='contact' && disabled!='true'` | Contact or already disabled |

#### 7. No eligibility rules needed for other tables (current behavior preserved)
- Groups bulk delete: no preconditions, all selected are eligible
- SSH certs bulk delete: no preconditions
- FIDO2 bulk revoke: no preconditions
- Service accounts bulk delete: no preconditions

### UX Flow Example (Delete)

1. Admin selects 25 users, bar shows "25 selected"
2. Admin clicks "Delete" button
3. JS evaluates: 20 eligible, 5 ineligible (active non-contacts)
4. 5 rows turn amber, bar shows "25 selected (5 ineligible)", button shows "Delete (20)"
5. Admin either:
   - Clicks "Delete (20)" again to confirm (deletes the 20, skips 5)
   - Deselects the 5 amber rows manually
   - Cancels and uses "Disable" on those 5 first, then re-runs delete

### UX Considerations

- [x] Selection state (JS Set) preserved throughout - no page reload loses it
- [x] Clicking a different bulk action button resets the confirmed state of other buttons
- [x] `conflict-row-bg` cleared when row is deselected or when action is cancelled
- [x] After action completes, table refreshes via HTMX and selection clears (existing behavior)
- [x] Flash message reports outcome: "20 users deleted (5 skipped - not disabled)"
- [x] Backend skip logic remains as safety net (defense in depth)

### Backend Changes (optional, improves feedback)

- [x] `actionBulkDeleteUsers`: return structured JSON with `deleted`, `skipped` counts
- [x] `actionBulkDisableUsers`: return structured JSON with `disabled`, `skipped` counts
- [x] Allows flash message to show precise outcome without client-side guessing

## Fix: Table Header Sort Not Applied (Users, Groups)

Sortable column headers fire HTMX requests with sort/order params but the partials
never applied the sort to the filtered data before paginating.

- [x] `partialUserList`: add `sort.Slice` on filtered rows using `state.Sort` and `state.Order`
- [x] `partialGroupList`: add `sort.Slice` on filtered rows using `state.Sort` and `state.Order`
- [x] Sort is case-insensitive for string fields (ToLower comparison)
- [x] SSH certs partial already sorts via SQL query (no change needed)

## Fix: Bootstrap Admin Missing employeeType

- [x] `createInitialAdmin` in `bootstrap.go` now sets `employeeType: "employee"` on the initial admin user

## Fix: OIDC Login Populates User Name from JWT

- [x] Add LDAP client to `AuthHandlers` struct
- [x] Extract `given_name` and `family_name` from ID token claims on callback
- [x] If user's givenName or sn is still the uid placeholder, update from JWT claims
- [x] Update cn to "givenName sn" when names are refreshed
- [x] Only updates on login if names are still placeholders (does not overwrite manual edits)

## Fix: Prevent Self-Deletion and Last-Admin Lockout

Admins can currently delete or disable their own account, locking themselves out permanently.
Standard fix: reject self-delete/self-disable and protect the last admin account.

### Self-Delete/Self-Disable Protection

#### API endpoints
- [x] `DELETE /api/v1/users/{uid}`: reject if uid matches authenticated principal (400: "cannot delete your own account")
- [x] `POST /api/v1/users/{uid}/disable`: reject if uid matches authenticated principal (400: "cannot disable your own account")

#### Frontend handlers
- [x] `actionDeleteUser`: reject if uid == emailToUID(claims.Email)
- [x] `actionDisableUser`: reject if uid == emailToUID(claims.Email)
- [x] `actionBulkDeleteUsers`: skip current user's UID, include in skipped count with reason
- [x] `actionBulkDisableUsers`: skip current user's UID, include in skipped count with reason

### Last-Admin Protection

- [x] Add helper: `CountActiveAdmins()` - query `authbox-admins` group members, count those not disabled
- [x] `actionDeleteUser`: if target is in `authbox-admins` and `countActiveAdmins() <= 1`, reject (400: "cannot delete the last admin")
- [x] `actionDisableUser`: same check
- [x] `actionBulkDeleteUsers`: skip last-admin with reason in skipped count
- [x] `actionBulkDisableUsers`: skip last-admin with reason in skipped count
- [x] API `deleteUser`: same last-admin check
- [x] API `disableUser`: same last-admin check

### Bulk action bar integration (nice-to-have)

- [x] Add `data-self="true"` to the current user's row checkbox
- [x] Add `data-admin="true"` to admin-group members' row checkboxes
- [x] Extend `EligibleIf` on Delete/Disable to exclude self: append `&& self!='true'`
- [x] Conflict row highlights the current user's row if selected for delete/disable

## Table Sort Persistence (localStorage)

Sort preferences reset to defaults when navigating away and back. Save sort/order
per table in localStorage so the user's preferred sort is restored on page load.

### Implementation

#### 1. Save sort state on every partial render (table.go RenderFooter JS)
- [x] After each partial swap, save `{sort, order}` to localStorage keyed by partial URL
- [x] Key format: `tableSort:<partialURL>` (e.g., `tableSort:/partials/users/list`)
- [x] Save happens in the existing `RenderFooter` script block (runs on every render)

#### 2. Restore sort state on initial table load (global HTMX listener)
- [x] Add a global `htmx:configRequest` event listener (in base layout template or static JS)
- [x] Listener checks if the request target is a `.table-container` element
- [x] If the request URL has no `sort` param and localStorage has a saved sort for that URL, append `sort` and `order` params to the request
- [x] Only applies to the initial `hx-trigger="load"` request (subsequent sort clicks already have params)

#### 3. Clear saved state when filter/search changes (optional)
- [x] When the filter input fires a request, do not override the saved sort (filter and sort are independent)
- [x] Page size changes should also preserve the saved sort (already passed via existing hx-include)

#### 4. Persist page size (limit) alongside sort
- [x] Include `limit` in the saved localStorage JSON: `{sort, order, limit}`
- [x] Restore `limit` in the `htmx:configRequest` listener (same guard as sort)
- [x] Page size select change triggers new request; saved sort still injected via listener (correct behavior)

### Notes

- Partial URL is unique per table, making it a natural localStorage key
- No server-side changes needed - this is purely client-side JS
- Pagination offset should NOT be saved (navigating back to page 37 is confusing)

## Bulk Action Bar: Eliminate Layout Shift

Selecting a checkbox reveals the bulk action bar, which pushes the table down causing a jarring layout jump. Fix by swapping the filter bar and bulk bar in the same fixed-height container.

### Implementation (table.go + style.css)

- [x] Wrap filter bar and bulk bar in a shared `.table-toolbar` container
- [x] Both bars rendered in DOM; bulk bar starts `hidden`, filter bar visible
- [x] `updateBulkBar()` JS: toggle `hidden` on both bars (show bulk, hide filter; and vice versa)
- [x] Add `.table-toolbar` CSS with `min-height` matching bar height so container never collapses
- [x] No changes to partials.go or table configs needed
- [x] Update tablerenderer.md steering file with toolbar swap pattern

## Backup Schedule: Full Implementation

The schedule form template exists but has no backend. Form POSTs to a non-existent route.

### 1. App Settings Storage (key-value table)

- [x] Add `app_settings` table to SQLite migrations: `key TEXT PRIMARY KEY, value TEXT NOT NULL`
- [x] Add repository methods: `GetSetting(key string) (string, error)`, `SetSetting(key, value string) error`
- [x] Include `app_settings` in backup export/restore (ExportData struct, RestoreState)

### 2. Fix Form Route and Add POST Handler

- [x] Fix `backup_schedule.html` form action from `/settings/backup-schedule` to `/backup/schedule`
- [x] Register `POST /backup/schedule` in admin route group (router.go)
- [x] POST handler: parse form (enabled bool, time string, retention int), validate, save to app_settings
- [x] Keys: `backup_schedule_enabled`, `backup_schedule_time`, `backup_schedule_retention`
- [x] Flash success "Backup schedule updated" and return updated partial

### 3. Load Persisted Settings in Partial Handler

- [x] `partialBackupSchedule` reads settings from DB instead of hardcoded defaults
- [x] Fallback to defaults if keys not set (enabled=false, time="02:00", retention=30)

### 4. Scheduler Goroutine

- [x] Add `internal/backup/scheduler.go` with `Scheduler` struct (repo, slapcatPath, backupDir, logger, stop chan)
- [x] `Start()`: read settings from DB, if enabled calculate next run time, start timer
- [x] On timer fire: run `ScheduledBackup()`, log result, re-arm for next day
- [x] `Reconfigure()`: called after POST handler saves new settings, resets timer
- [x] `Stop()`: clean shutdown (called on app exit)
- [x] Handle disabled state: if not enabled, timer is not armed (no-op until reconfigured)

### 5. Wire Into main.go

- [x] Create Scheduler after DB init, pass to frontend Deps (so POST handler can call Reconfigure)
- [x] Call `scheduler.Start()` before HTTP server starts
- [x] Call `scheduler.Stop()` on shutdown

### 6. Status Integration

- [ ] Show last backup time on backup page (read latest file timestamp from backup dir)
- [ ] Show next scheduled run time (calculated from settings)

### 8. Backup Schedule Form UX

- [x] Disable time/retention inputs when checkbox is unchecked (JS `onchange` toggles `disabled`)
- [ ] Button text is static "Save Schedule" (no dynamic text changes)
- [ ] Remove JS that swaps button text on checkbox change
- [ ] Remove template conditional around button text

### 9. Disabled Field Styling (global, style.css)

- [x] Add `.input:disabled` rule: `opacity: 0.5; cursor: not-allowed;`
- [x] Add `.label` grey text when sibling input is disabled (wrap label+input in `.field-group`, toggle `.disabled` class via JS)
- [x] Dark mode variant: disabled label uses `#6b7280` (gray-500)
- [x] Update `toggleScheduleFields()` in backup_schedule.html to toggle `.disabled` on parent divs
- [x] Applies globally to any future form with conditional disabled fields

### 7. Backup Archives Table (table renderer)

Add an "Archives" sidebar item to browse/manage backup files in `/data/backups/`.

#### Table config
- [x] Columns: Filename (sortable), Type (scheduled/pre-import, not sortable), Size (sortable), Date (sortable), _actions
- [x] Filterable (search by filename)
- [x] Selectable with bulk delete action (confirm required)
- [x] Data source: `os.ReadDir` on backup directory, stat each file

#### Partial and routes
- [x] Add `backup_archives.html` partial template (just the table-container div)
- [x] Add partial handler `partialBackupArchives` - reads dir, builds slice, filter/sort/paginate via TableRenderer
- [x] Register `GET /partials/backup/archives` in partial routes
- [x] Register `GET /backup/archives-panel` to serve the partial wrapper (sidebar loads this)
- [x] Add "Archives" nav item to backup sidebar config

#### Per-row actions
- [x] `GET /backup/archives/{filename}` - stream file download (Content-Disposition: attachment)
- [x] `POST /backup/archives/{filename}/delete` - remove file, return refreshed partial via HX-Trigger
- [x] `POST /backup/archives/{filename}/import` - stage LDIF to `/data/live-restore/`, require yesiagree confirm, trigger restart

#### Bulk actions
- [x] `POST /backup/archives/bulk/delete` - bulk delete selected files (confirm required)

#### Type detection
- [x] Files prefixed `pre-import-backup-` = "Pre-import"
- [x] Files prefixed `backup-` = "Scheduled"
- [x] All others = "Manual"

## Backup Schedule API Endpoints

- [ ] `GET /api/v1/config/backup-schedule` - return current schedule settings (enabled, time, retention)
- [ ] `PUT /api/v1/config/backup-schedule` - update schedule settings, call `scheduler.Reconfigure()`
- [ ] `GET /api/v1/config/backups` - list backup archive files (filename, size, modified, type)
- [ ] `DELETE /api/v1/config/backups/{filename}` - delete a specific backup archive

## Fix: Backup Scheduler Reconfigure Does Not Wake Loop

`Reconfigure()` stops the old timer and assigns a new zero-duration timer to `s.timer`, but the `loop()` goroutine's `select` is already blocked on the old timer's channel. Replacing the struct field doesn't unblock the select. The loop won't re-read settings until the old timer would have fired (up to 24h) or the idle sleep expires (1h).

- [x] Add `reconfigCh chan struct{}` field to `Scheduler` (unbuffered or capacity 1)
- [x] Initialize `reconfigCh` in `NewScheduler`
- [x] Add `case <-s.reconfigCh:` to the select in `loop()` (alongside stopCh and timer.C)
- [x] When reconfigCh fires, stop the current timer and `continue` the loop (re-reads settings, recomputes duration)
- [x] In `Reconfigure()`, send on `reconfigCh` (non-blocking) instead of manipulating `s.timer` directly
- [x] Remove the two-phase lock/unlock and `time.NewTimer(0)` hack from `Reconfigure()`
- [ ] Verify: saving a new schedule time takes effect within seconds, not on next loop iteration
