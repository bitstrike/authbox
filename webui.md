# Authbox Web Interface

All pages require OIDC authentication unless noted. Role-based access controls which features are visible.

## Navigation

- Dashboard
- Users
- Groups
- SSH Certificates
- FIDO2 Keys
- Service Accounts
- Logs
- Settings
- Backup
- Status indicator in header (green = healthy, warning icon = issues, red alert = errors)

## Session

- Web UI session timeout: 30 minutes (configurable in Settings)
- Expired session redirects to login page
- Activity resets the timeout

## Destructive Action Confirmation

Potentially catastrophic actions require typing a confirmation word (e.g., "yesiagree") before proceeding. This applies to:
- Re-enabling a disabled user account
- Importing a backup (overwrites current directory)
- Deleting a group with members
- Deleting a service account
- Any future action that could cause data loss or security impact

## Pages

### Login

- Unauthenticated
- "Sign in with Google" or "Sign in with Microsoft" button (based on configured IdP)
- Redirects to IdP, returns to dashboard on success
- Rejects login if user does not exist in LDAP directory
- Users with only "self" role can issue their own SSH certs and register their own FIDO2 keys
- All other account changes require operator or admin (handled through change management)

### Dashboard

- Roles: all authenticated users
- Summary stats: total users, active users, disabled users, groups, certs issued today
- Recent activity log (last 20 events)
- System status: LDAP connection, replica sync state, cert expiry warnings

### Users

- Roles: viewer (read), operator (create/edit/disable), admin (all)

#### User List
- Paginated table: username, full name, email, uidNumber, status (active/disabled)
- Search/filter by name, email, status
- Bulk import button (CSV or JSON upload)
- Create user button

#### Create User
- Form fields: uid, givenName, sn, mail, uidNumber (optional - auto-assign if blank), gidNumber (optional), homeDirectory, loginShell
- Validation: uniqueness check on uid and uidNumber before submit
- Shows assigned uidNumber/gidNumber in confirmation

#### Edit User
- Same fields as create (uid is read-only)
- Change uidNumber/gidNumber (with conflict check)
- Group membership display and edit
- FIDO2 credentials listed (link to FIDO2 page)
- SSH certs issued to this user (link to SSH page)

#### Disable User
- Confirmation dialog: "This will set shell to /sbin/nologin, remove from all groups, and revoke FIDO2 credentials"
- Shows what will change before confirming
- Cannot be undone from this page (re-enable is a separate action)

#### Re-enable User
- Roles: admin only
- Requires typing "yesiagree" to confirm (destructive action confirmation)
- Restores loginShell to previous value
- Does NOT automatically re-add to groups or restore FIDO2 credentials (must be done manually)
- Audit logged

#### Bulk Import
- Upload CSV or JSON file
- CSV columns: uid, givenName, sn, mail, uidNumber (optional), gidNumber (optional), homeDirectory, loginShell
- JSON: array of user objects with same fields
- Preview table before confirming import
- Shows errors (conflicts, missing fields) inline
- Skips or flags duplicates

### Groups

- Roles: viewer (read), operator (edit membership), admin (create/delete)

#### Group List
- Paginated table: cn, type (posixGroup or groupOfNames), gidNumber, member count
- Filter by type

#### Create Group
- Form: cn, type (posixGroup or groupOfNames), gidNumber (optional for posixGroup)
- Type selection determines available fields

#### Edit Group
- Add/remove members
- For posixGroup: memberUid list
- For groupOfNames: member DN list
- Searchable user picker for adding members

#### Delete Group
- Confirmation dialog
- Cannot delete authbox role groups (authbox-admins, authbox-operators, authbox-viewers) unless empty

### SSH Certificates

- Roles: self (own certs), viewer (all certs), admin (all)

#### Issue Certificate
- Available to all authenticated users (self role)
- Text area: paste public key (ssh-ed25519, ssh-rsa, ecdsa-sha2)
- Displays configured TTL (from SSH_CERT_TTL)
- Submit: returns signed certificate for download
- Shows certificate details: principal, valid from/to, serial

#### Certificate List
- Roles: viewer, admin
- Paginated table: username, serial, issued at, expires at, principal
- Filter by user, date range
- Audit view: who issued what, when

### FIDO2 Keys

- Roles: self (own keys), operator (any user), admin (any user)

#### Register Key
- Instructions displayed: "On your workstation, run: `pamu2fcfg -n`"
- Text area: paste the output
- Validates format before storing
- Associates with current user (self) or specified user (operator/admin)
- Supports registering multiple keys per user (primary + backup)

#### Key List
- Per-user view: shows registered credentials
- Delete/revoke individual credentials
- Indicator: number of keys registered (warn if only 1 - recommend backup)

### Service Accounts

- Roles: admin only

#### Service Account List
- Table: client_id, description, role, created date, last used

#### Create Service Account
- Form: description, role (viewer, operator, admin)
- On submit: displays client_id and client_secret once
- Warning: "Save the secret now. It cannot be retrieved again."

#### Delete Service Account
- Confirmation dialog
- Immediately revokes all tokens issued to this account

### Settings

- Roles: admin only

#### OIDC Provider
- Current provider display (Google or Entra)
- Form: issuer URL, client ID
- Client secret: shows "configured" (not the value), button to update (reads from secrets volume)
- Test connection button

#### Session
- Session timeout duration (default: 30 minutes, configurable)

#### LDAP Configuration
- Base DN (read-only after init)
- View current ACLs
- Replication settings (primary/replica status, peer address)
- TLS certificate status and expiry

#### UID/GID Range
- Current range display (start/end)
- Next available UID/GID
- Edit range

#### SSH CA
- CA public key display (copyable)
- CA key fingerprint
- Default cert TTL (editable)

#### Logging
- Current log level (editable: debug, info, warn, error)
- Log retention days (default 90, editable)
- View recent logs (tail)

### Logs

- Roles: admin only

#### Log Viewer
- List of available log files (by date)
- Paginated viewer for historical logs
- Live-tail mode for current log (auto-scrolling, real-time updates)
- Filter by level (info, debug, warn, error)
- Search within log content

### Status and Alerts

- Roles: all authenticated users (visible in header)

#### Status Indicator (Header)
- Green checkmark: all systems healthy
- Warning icon (yellow): non-critical issues detected
- Red alert icon: critical errors requiring attention
- Clicking the indicator navigates to the status page

#### Status Page
- LDAP connection status
- Replica sync state (last sync time, lag)
- TLS certificate expiry (warns at 30 days)
- Backup status (last successful backup, any failures)
- Disk usage on persistent volumes
- List of active warnings and errors with timestamps

### Backup

- Roles: admin only

#### Export
- Button: "Export Now" - triggers full export (LDAP directory, cn=config, FIDO2 mappings, SQLite)
- Download as archive file
- Schedule daily slapcat backup (toggle on/off)
- Backup time configuration (default: 02:00 local)
- Retention for scheduled backups (default: 30 days)
- Backup storage path (within persistent volume)

#### Import
- Upload export archive
- Preview what will be imported
- Confirmation: "This will overwrite the current directory"
- Requires admin role

#### CA Key Backup
- Instructions for manual CA key backup
- CA key fingerprint for verification
- Note: CA key is NOT included in standard export (must be backed up separately)

## Theme

- Light and dark mode
- Respects system preference (`prefers-color-scheme`)
- Manual toggle in navigation header
- Preference persisted in browser localStorage
