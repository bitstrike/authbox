# Authbox

<img src="images/authbox.png" alt="Authbox" width="40%">

Authbox is a centralized authentication and authorization container for Linux systems built on OpenLDAP and Go. A web frontend provides dashboard and management of users and groups through Google (tested) or Microsoft Entra ID (untested) OIDC authentication.

<img src="images/shot-1.png" alt="Dashboard" width="40%">

Centralized password authentication on Linux isn't great. There are a lot of options but nothing that's really a drop-in auth package with a friendly way to manage the fiddly bits of OpenLDAP. This project is intended to experiment with some other ways of providing this. I've spent decades working with OpenLDAP in production environments and the documentation has always been the first groan. It's there, but it's terse and always leaves you with more questions. Second gron is the mailing list support. Sometimes you get an answer, most of the time not. Third groan is the configuration complexity. Punching out an LDIF for the right thing and then feeding it into `ldap*` commands to either add or modify is tedious and the error messages are hardly helpful when you have a typo or other. For all it's complexity however, I've never had it fall over once setup correctly. It just runs. It's likely one of the most stable pieces of software I've ever worked with. 

All this being said, I thought building an interface around the most common features I've used OpenLDAP for seemed like a neet idea. Sun and Redhat tried this back in the 90s but they were slow and miserable java applications which I could never get running suitably on the hardware I had available. I guess this project is similar in theme but hopefully a lot less miserable. It is certainly a work in progress right now. I am running this in a homelab environment so most features are getting regular exercise. The primary/secondary feature is untested yet but this isn't terribly difficult to get working with OpenLDAP - I'm just opting to get all the features working/tested on the primary first.

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
| `TZ` | `UTC` | Timezone (must be IANA format, e.g. `America/Chicago`) |
| `LOG_LEVEL` | `info` | Log level: debug, info, warn, error |
| `LOG_DIR` | `/app/logs` | Log output directory |

**Timezone note:** The `TZ` variable must be a valid IANA timezone name (e.g. `America/Chicago`, `US/Eastern`). POSIX-style values like `CST6DST` are not recognized by Go's time library and will silently fall back to UTC.

### Secrets Directory

Secrets are plain files on the **Docker host** at `/etc/secrets/authbox/`. The container bind-mounts this directory read-only (see `docker-compose.yml`). We don't pass these to docker as arguments since that can expose the secrets to docker inspect or other processes within the container.

Create the directory and populate it before starting the container. The container currently runs as root but planned TODO is dropping privs from the entrypoint so group read may be needed. For now, `root:root` is where it's at.  `slapd` will drop privs to the ldap user on startup so that part we get for free. The authbox process is running as `root` currently but this shoudl get dropped privs on it too.  There are some setup tasks before doing this to make sure the Go process can access the tls directory as well as read the needed files.  I haven't thought through all this yet because this whole project started as this nebulous PoC and it's slowly evolving into something possibly usable so the privs are on the roadmap.

```bash
sudo mkdir -p /etc/secrets/authbox
sudo chmod 750 /etc/secrets/authbox

# Create each file with appropriate content (see below)
sudo tee /etc/secrets/authbox/ldap_admin_password <<< "your-ldap-password"
sudo tee /etc/secrets/authbox/replica_sync_password <<< "your-sync-secret"
# ... google, entra, aws as needed

# make sure things are adequately permissioned
sudo chown -R root:root /etc/secrets/authbox
sudo find /etc/secrets/authbox -type d -exec chmod 750 {} \;
sudo find /etc/secrets/authbox -type f -exec chmod 640 {} \;
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
CLIENT_ID=123456789-abc.apps.googleusercontent.com
CLIENT_SECRET=GOCSPX-xxxxx
```

#### `/etc/secrets/authbox/entra`

Microsoft Entra ID credentials. Key names follow Azure SDK convention. Use this instead of `google` (only one IdP active at a time).

```
AZURE_CLIENT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
AZURE_CLIENT_SECRET=xxxxxxxx
AZURE_TENANT_ID=xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
```

#### `/etc/secrets/authbox/aws`

AWS credentials for Route53 DNS-01 Let's Encrypt automation (see project `terraform` directory).  Key names follow `~/.aws/credentials` format.

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

| Port | Protocol | Exposure | Purpose |
|---|---|---|---|
| 389 | LDAP+STARTTLS | External | POSIX identity lookups (nslcd) |
| 636 | LDAPS | External | Legacy LDAP over TLS |
| 3389 | LDAP (plain) | Internal (127.0.0.1 only, not published) | Go app to local slapd communication |
| 8443 | HTTPS | External | Web UI and REST API |

In an HA deployment the replica container publishes these on different host ports to avoid collisions (`390`, `637`, `8444` -> container `389`, `636`, `8443`); see `docker/docker-compose.yml`.

## TLS Certificates

Authbox automatically manages TLS certificates. On first boot:

- If `TLS_DOMAIN` is set: obtains a Let's Encrypt certificate via DNS-01 challenge before starting services. No inbound connectivity required.
- If `TLS_DOMAIN` is empty: generates a self-signed certificate for development/testing.

Certificates are stored on the persistent `/data` volume and reused across restarts. Renewal runs automatically 30 days before expiry.

**DNS provider:** Currently uses AWS Route53 for DNS-01 challenges. Requires `AWS_HOSTED_ZONE_ID` env var and AWS credentials in `/etc/secrets/authbox/aws`. The IAM user/role needs only:

```json
{
  "Effect": "Allow",
  "Action": [
    "route53:ChangeResourceRecordSets",
    "route53:GetChange"
  ],
  "Resource": "arn:aws:route53:::hostedzone/YOUR_ZONE_ID"
}
```

Planned conversion to the [lego](https://github.com/go-acme/lego) library to support Cloudflare, Google Cloud DNS, and 100+ other providers.

**Provisioning the IAM user with OpenTofu:**
If you don't want to click around the AWS console to create the needed IAM stuff, I've included a `terraform/` directory that contains an OpenTofu configuration that probably creates the IAM user, policy, and access key. I say "probably" because this is a stripped down copy of the terraform I used to configure my IAM. `main.tf` is the same but I'm not including my `tfvars` so adapted a launch script that will get your variables out of `pass` which is what I advocate for. You can use the included `tf-launch` script or launch `tf` by hanhd. The days of clear-text internal secrets is over so get used to it. Store variables in `pass` then run:

```bash
pass insert authbox/terraform/region
pass insert authbox/terraform/hosted_zone
pass insert authbox/terraform/domain_name
pass insert authbox/terraform/iam_user_name
```

Then:
```bash
./terraform/tf-launch.sh plan
./terraform/tf-launch.sh apply
```

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

## Client Configuration

To authenticate users on a Linux host against authbox, three things are configured: name resolution (NSS), SSH certificate trust, and optionally FIDO2 for console login.

### Files Modified

| File | Purpose |
|---|---|
| `/etc/nslcd.conf` | Points nslcd at authbox's LDAP (host, base DN, TLS) |
| `/etc/nsswitch.conf` | Adds `ldap` to passwd/group/shadow lookups |
| `/etc/ssh/trusted_ca.pub` | SSH CA public key (fetched from authbox API) |
| `/etc/ssh/sshd_config` | `TrustedUserCAKeys /etc/ssh/trusted_ca.pub` |
| `/etc/pam.d/u2f-auth` | PAM config for FIDO2 hardware key login (optional) |
| `/etc/u2f_mappings` | FIDO2 credential mappings synced from authbox (optional) |

### What Each Layer Does

- **NSS (nslcd)** - Resolves LDAP users/groups system-wide. `getent passwd`, `ls -l`, `id username` all work with LDAP entries.
- **SSH CA trust** - Users sign their pubkey via authbox web UI, then SSH in without per-host key distribution. No passwords involved.
- **FIDO2 (pam_u2f)** - Physical console/GDM login using a YubiKey. Works fully offline.

### Automated Setup (Ansible)

The included playbook configures all three layers. Requires Ansible on a control machine with SSH access to the target host:

```bash
ansible-playbook ansible/playbooks/enroll-host.yml \
  -i "target-host," \
  -e platform_host=authbox.example.com \
  -e base_dn=dc=example,dc=com \
  --become
```

Or.. if you have root access to the remote host over ssh already..
```bash
ansible-playbook ansible/playbooks/enroll-host.yml \
  -i "target-host," \
  -u root \
  -e platform_host=authbox.example.com \
  -e base_dn=dc=example,dc=com \
  -e ansible_become=false
```

Replace `target-host` with the hostname or IP, and adjust `platform_host` and `base_dn` for your environment.

To sync FIDO2 mappings (run periodically or after key enrollment):

```bash
LINUX_AUTH_TOKEN=<service-account-token> \
ansible-playbook ansible/playbooks/sync-fido2-mappings.yml \
  -i "target-host," \
  -e platform_host=authbox.example.com \
  --become
```

### Ansible Variables

Variables consumed by `enroll-host.yml`. Required variables have no default and
must be supplied via `-e` or inventory; the rest have defaults shown below.

| Variable | Required | Default | Purpose |
| --- | --- | --- | --- |
| `platform_host` | yes | - | Platform hostname/IP. Derives `platform_url` (`https://<host>:8443`) and `ldap_uri` (`ldap://<host>`). |
| `base_dn` | yes | - | LDAP base DN (e.g. `dc=example,dc=com`). Sets `ldap_base_dn`. |
| `ssh_enforce_cert_validation` | no | `false` | Enable SSH cert validation: deploys cert-check + cache-refresh scripts, `AuthorizedPrincipalsCommand`, and the cert-cache cron job. |
| `ssh_cert_cache_interval` | no | `5` | Cert-cache refresh interval, minutes (integer 1-59). Used as the cron `*/N` minute field. |
| `ssh_kill_disabled_sessions` | no | `false` | Enable session revocation: deploys the session-check script and cron job that terminates SSH sessions of disabled users. |
| `ssh_session_check_interval` | no | `1` | Session-check interval, minutes (integer 1-59). Used as the cron `*/N` minute field. |

Environment variables (read by the playbook, required only when
`ssh_enforce_cert_validation: true`):

| Env var | Purpose |
| --- | --- |
| `CERT_REFRESH_CLIENT_ID` | Viewer-role service account client ID, written to the host credentials file for the cache-refresh script. |
| `CERT_REFRESH_CLIENT_SECRET` | Service account client secret, paired with the client ID. |

### Manual Verification

After enrollment, verify NSS resolution:

```bash
getent passwd          # should list LDAP users
getent group           # should list LDAP posixGroups
id someuser            # should resolve a provisioned user
```

### Notes

- Only users with `posixAccount` (UID/GID > 0) are visible via NSS. Contacts do not appear.
- SSH cert auth requires no local password. The user signs their key through the web UI or API.
- PAM password authentication is not supported (authbox uses OIDC, not stored passwords). Use SSH certs or FIDO2.
- Alpine Linux uses different package names (`nss-pam-ldapd`, `pam-u2f`, `openssh`). The playbook handles this automatically.

### SSH Login Roles: how they work and how to set one up

An SSH login role lets a user log into a shared account (e.g. `ops`) on enrolled
hosts, while the certificate still records the real human in its `KeyId` for audit.
There is no new step for the user: the role rides along as an extra principal on
their normal certificate.

**How it works (the invisible chain)**

- Membership in a `sshrole-<name>` group causes `<name>` to be stamped onto the
  user's certificate as an extra principal, alongside their own uid principal.
  Example: a member of `sshrole-ops` gets a cert with principals `alice, ops`.
- On the host, `AuthorizedPrincipalsCommand` (`authbox-cert-check.sh`) authorizes
  login as an account only if the cert carries a principal equal to that account
  name (same-name convention). Self-login (`ssh alice@host` with principal `alice`)
  always works.

**How to set one up (admin)**

- Create a `groupOfNames` named `sshrole-<name>` (e.g. `sshrole-ops`) under
  `ou=groups` using the existing Groups UI/API.
- Name constraint: `[a-z0-9-]` only. The name becomes both an SSH principal and a
  Unix account name; a name outside this charset silently never becomes a principal.
- Ensure the target account exists and resolves on every host. Preferred: create it
  as an LDAP posix user (served by `nslcd`) so it exists fleet-wide with no local
  account creation.
- Add users (member DNs) to the group. The change takes effect on their next
  certificate issuance - roles are only valid for the life of the cert that carries
  them.

**Using it (user)**

- Get added to the role group by an admin.
- Sign your key as usual (no extra step).
- `ssh <role>@host` (e.g. `ssh ops@host`). `ssh <you>@host` still works too.

**Security notes and gotchas**

- Roles map to same-name shared accounts only. There is no mapping to `root` by
  design - use `sudo` to elevate after login. Root SSH-key login for automated
  backups is a separate path, untouched by roles.
- App/API roles (`authbox-admins`/`operators`/`viewers`) are unrelated and
  deliberately decoupled: being an app admin grants no host login.
- Certificates are short-lived. Removing someone from a role takes effect when their
  current cert expires (there is no live revocation).
- If `ssh_enforce_cert_validation` is enabled, the host's valid-serials cache must
  have refreshed since issuance, or the new serial is not yet authorized.

### Per-Host Access Control

By default, any provisioned user with a valid SSH certificate can log into any
enrolled host (the certificate is trusted fleet-wide). To restrict which users may
log into which hosts, gate login on the host using an LDAP group. This is a standard
Linux login control using a `posixGroup` managed through the normal group UI/API.

The model is one access group per host (or per host tier), holding the users allowed
there. Example: `bob` may access `host-1`, `host-2`, `host-3`; `alice` may access
`host-1` only.

Create the access groups as **posixGroups** (they need a `gidNumber` and use
`memberUid`, so they resolve via `getent group`):

```
cn=hostaccess-1,ou=groups   memberUid: bob, alice
cn=hostaccess-2,ou=groups   memberUid: bob
cn=hostaccess-3,ou=groups   memberUid: bob
```

Choose one of the two enforcement options below. Both produce the same result:

| User  | host-1 | host-2 | host-3 |
|-------|--------|--------|--------|
| bob   | allow  | allow  | allow  |
| alice | allow  | deny   | deny   |

Prerequisite for either option: on each host, `getent group hostaccess-1` must list
the members. If it does not, NSS is not resolving the group and neither mechanism can
gate on it.

#### Option 1: `sshd AllowGroups` (SSH-only, simplest)

`AllowGroups` matches the user's Unix group membership and gates SSH login only.
Set the host's own access group in `/etc/ssh/sshd_config`, and keep an admin group
so you are not locked out:

`host-1` `/etc/ssh/sshd_config`:
```
AllowGroups hostaccess-1 wheel
```
`host-2`:
```
AllowGroups hostaccess-2 wheel
```
`host-3`:
```
AllowGroups hostaccess-3 wheel
```

On `host-2`, sshd only admits members of `hostaccess-2` (bob), so alice is rejected at
the SSH layer before a session starts. `wheel` (or your admin posixGroup) stays allowed
so administrators keep access.

#### Option 2: `pam_access` (gates all PAM logins: SSH, console, GDM)

This option is necessary whenever a host has any login path other than SSH. In the
authbox model, SSH is authenticated by certificate, but **local console login is
authenticated by FIDO2 (`pam_u2f`)**, which `enroll-host.yml` wires into
`/etc/pam.d/login`, and GDM desktop login is planned to use the same. Those are
separate doors into the host that `sshd_config AllowGroups` (Option 1 above) does not touch.

Option 1 restricts `ssh alice@host-2`, but if alice has physical or console access to host-2
and a registered YubiKey, she can still log in at the text console because that path never goes
through sshd. To actually keep alice off host-2, the gate must sit where every login path converges,
whic ends up being pam_access.

`pam_access` runs in the PAM **account** phase, which is evaluated on every PAM login
(SSH, console/`login`, GDM) regardless of how the user authenticated.
Authentication and authorization are separate phases: FIDO2 proves who the user is in
the **auth** phase (the YubiKey touch), then `pam_access` independently decides in the
**authorization** phase whether that identity is allowed on this host. A denied user still
completes the touch, then the login is refused. Because the deny happens before the **session**
phase, `pam_mkhomedir` never runs for a denied user, so no stray home directory is created.

Use Option 2 (optionally alongside Option 1) when the host allows console or GDM login,
which in the authbox FIDO2 model is every host with local access. Use Option 1 alone
only for headless hosts where SSH is genuinely the sole login path.

`pam_access` matches a group via the `(name)` syntax and applies to every PAM login
path, not just SSH. Use the same posixGroups as Option 1.

Because the goal is to cover console and GDM in addition to SSH, place the check in the
shared account stack rather than only in `/etc/pam.d/sshd`. On Debian/Ubuntu that is
`/etc/pam.d/common-account` (pulled in by `login`, `sshd`, and `gdm-password`); on
RHEL-family it is `/etc/pam.d/system-auth`. Adding it there gates all login paths at
once:
```
# /etc/pam.d/common-account (Debian)  or  /etc/pam.d/system-auth (RHEL)
account required pam_access.so
```
If you prefer to gate only specific paths, add the same line to each of
`/etc/pam.d/login` (text console), `/etc/pam.d/gdm-password` (GDM), and
`/etc/pam.d/sshd` (SSH) instead. Adding it to `sshd` alone reproduces Option 1's
SSH-only coverage and leaves the FIDO2 console door open.

Add the check to the SSH PAM stack (account phase) in `/etc/pam.d/sshd`:
```
account required pam_access.so
```

Then write the host's `/etc/security/access.conf`. Rules are evaluated top-down,
first match wins; the trailing deny-all rejects everyone not explicitly allowed.
Keep `root` (and any local admin) allowed to avoid lockout.

`host-1` `/etc/security/access.conf`:
```
+ : root : ALL
+ : (hostaccess-1) : ALL
- : ALL : ALL
```
`host-2`:
```
+ : root : ALL
+ : (hostaccess-2) : ALL
- : ALL : ALL
```
`host-3`:
```
+ : root : ALL
+ : (hostaccess-3) : ALL
- : ALL : ALL
```

On `host-2`, alice authenticates but the account phase denies her (she is not in
`hostaccess-2`, so she hits `- : ALL : ALL`). bob matches `(hostaccess-2)` and is
allowed.

#### Choosing between them

- **Option 1 (`AllowGroups`)**: one config line per host, SSH-only. Safe only on headless
  hosts with no console or GDM access. On any host with local access it leaves the FIDO2
  console/GDM login path ungated.
- **Option 2 (`pam_access`)**: gates SSH, console, and GDM logins in the account phase, so
  it covers the FIDO2 console path that Option 1 misses. Costs a PAM line plus a config
  file. This is the option to use on any host a user could log into locally.

Both gate *login*, not *name resolution*. A denied user is still resolvable by `getent`
(so file ownership still shows names) but cannot log in - the same posture as a disabled
account.

#### Applying it with Ansible

The access configuration lives in two different places. One in LDAP, the other in Ansible to setup
pam_access and sshd_config.

- **Who is allowed** lives in LDAP group membership (managed through the authbox group UI).
- **Which groups a host honors** lives in the Ansible inventory as a per-host (or per-tier) variable.

For the ansible part, on way to define things is as an allow list of groups plus a separate admin group that is always merged in. This way, a typo in the group list doesn't lock out admin (assuming they are in the wheel group).
Leave the list empty to fail open so hosts already enrolled stay working.

Ansible inventory for the `bob`/`alice`, `host-1`/`host-2`/`host-3` scenario (bob everywhere,
alice on host-1 only):

```ini
# inventory.ini
[host1]
10.0.0.1 authbox_access_groups='["hostaccess-1"]'

[host2]
10.0.0.2 authbox_access_groups='["hostaccess-2"]'

[host3]
10.0.0.3 authbox_access_groups='["hostaccess-3"]'

[all:vars]
authbox_admin_group=wheel
```

Membership is set via LDAP posix groups:

```
cn=hostaccess-1,ou=groups   memberUid: bob, alice
cn=hostaccess-2,ou=groups   memberUid: bob
cn=hostaccess-3,ou=groups   memberUid: bob
```

**Option 1** (`AllowGroups`, SSH-only). `validate` runs `sshd -t` against the
candidate file so a bad line fails the task instead of breaking sshd:

```yaml
- name: Restrict SSH logins to the host's access groups
  ansible.builtin.lineinfile:
    path: /etc/ssh/sshd_config
    regexp: '^AllowGroups\s'
    line: "AllowGroups {{ (authbox_access_groups + [authbox_admin_group]) | join(' ') }}"
    validate: "sshd -t -f %s"
  notify: restart sshd
  when: authbox_access_groups | length > 0
```

This renders per host from the same task: `host-1` gets
`AllowGroups hostaccess-1 wheel`, `host-2` gets `AllowGroups hostaccess-2 wheel`, and so
on. Groups OR-match, so listing several (e.g. `["web","platform-admins"]`) grants access
to members of any of them.

**Option 2 task** (`pam_access`, gates SSH + console + GDM). Template `access.conf` from
the same variable and enable the module in the shared account stack:

```yaml
- name: Deploy pam_access rules for this host
  ansible.builtin.template:
    src: templates/access.conf.j2
    dest: /etc/security/access.conf
    owner: root
    group: root
    mode: "0644"
  when: authbox_access_groups | length > 0

- name: Enable pam_access in the shared account stack
  ansible.builtin.lineinfile:
    path: "{{ '/etc/pam.d/common-account' if ansible_os_family == 'Debian' else '/etc/pam.d/system-auth' }}"
    line: "account required pam_access.so"
    insertafter: EOF
  notify: restart sshd
  when: authbox_access_groups | length > 0
```

```jinja
{# templates/access.conf.j2 #}
+ : {{ authbox_admin_group }} : ALL
{% for grp in authbox_access_groups %}
+ : ({{ grp }}) : ALL
{% endfor %}
- : ALL : ALL
```

To scale, prefer grouping hosts by tier and setting the variable on the tier instead of
each host, so adding a host is a one-line inventory change with no new group:

```ini
[web]
10.0.0.1
10.0.0.2
[db]
10.0.0.3

[web:vars]
authbox_access_groups='["web-team","platform-admins"]'
[db:vars]
authbox_access_groups='["db-team","platform-admins"]'
```

Note: these tasks are not yet part of `enroll-host.yml`; the snippets above are the
reference implementation. Every group named must be a resolvable `posixGroup`
(`memberUid`), or `AllowGroups`/`pam_access` will silently match nobody.

### FIDO2 Console Login

FIDO2 lets a user log in at the physical console or GDM with a YubiKey. It is
independent of SSH (SSH uses certificates, not the key). Setup has four parts:
enroll the key, sync the mapping to the host, wire PAM, and test.

#### 1. Enroll the key

On the machine with the YubiKey attached, run:

```bash
pamu2fcfg -n -o pam://authbox -i pam://authbox
```

Paste the output into the authbox FIDO2 page. Leave the User (uid) field blank
to enroll for yourself; an admin may set another uid.

Notes:
- `-n` blanks the username field; the server assigns the uid. The server strips
  the leading colon this produces, so the stored mapping is `uid:keyhandle,...`.
- `-o`/`-i` pin a fixed origin (`pam://authbox`). This is required. Without it,
  `pamu2fcfg` and pam_u2f default the origin to `pam://<hostname>`, so a key
  enrolled on one host will not validate on another. The PAM config uses the
  same `origin=pam://authbox appid=pam://authbox`, so they must match.

#### 2. Sync the mapping to the host

Enrollment only stores the credential in authbox. Push it to the host's
`/etc/u2f_mappings` with the sync playbook. It needs a service-account bearer
token (see below for minting one):

```bash
LINUX_AUTH_TOKEN=<service-account-token> \
ansible-playbook ansible/playbooks/sync-fido2-mappings.yml \
  -i "target-host," \
  -e platform_host=authbox.example.com \
  --become
```

Mint a token from a service account (client_id/client_secret). The `client_id` and `client_secret` are obtained when you create the Authbox service account.:

```bash
read -s CLIENT_ID
read -s CLIENT_SECRET
export LINUX_AUTH_TOKEN=$(curl -sk -X POST \
  https://authbox.example.com:8443/oauth/token \
  -d grant_type=client_credentials \
  -d client_id=$CLIENT_ID \
  -d client_secret=$CLIENT_SECRET | jq -r .access_token)
```

Service tokens are in-memory with a 1-hour TTL and are invalidated on server
restart, so re-mint if you get `401 Unauthorized`.

Verify on the host (single colon, keyhandle first):

```bash
grep '^someuser:' /etc/u2f_mappings
```

#### 3. PAM wiring

`enroll-host.yml` deploys `/etc/pam.d/u2f-auth` and includes it into the console
login stack (`@include u2f-auth` at the top of `/etc/pam.d/login`). The rule is:

```
auth [success=done default=ignore] pam_u2f.so authfile=/etc/u2f_mappings origin=pam://authbox appid=pam://authbox cue
```

- `[success=done default=ignore]` - a successful key ends the auth stack (no
  password prompt). On failure it falls through to `common-auth`, leaving a
  local password path for recovery.
- `cue` - prompts the user to touch the key.

#### 4. Touch-only vs PIN

Two modes are supported. The credential enrollment and the PAM line must match:
a touch-only credential fails against a PIN-required PAM line with
`Unsupported options, skipping authenticator`, and vice versa.

**Touch-only (default).** Login requires only a physical touch of the key.

Enroll:

```bash
pamu2fcfg -n -o pam://authbox -i pam://authbox
```

PAM line (`ansible/files/pam-u2f-auth`):

```
auth [success=done default=ignore] pam_u2f.so authfile=/etc/u2f_mappings origin=pam://authbox appid=pam://authbox cue
```

**PIN + touch.** Login requires the key's FIDO2 PIN and a touch. The credential
must be enrolled with PIN verification.

Set a PIN on the key, then enroll with PIN required (`-N`; confirm the flag with
`pamu2fcfg --help`):

```bash
ykman fido access change-pin                     # set the PIN if not already set
pamu2fcfg -n -N -o pam://authbox -i pam://authbox
```

PAM line (`ansible/files/pam-u2f-auth`), add `pinverification=1`:

```
auth [success=done default=ignore] pam_u2f.so authfile=/etc/u2f_mappings origin=pam://authbox appid=pam://authbox pinverification=1 cue
```

The PIN is the key's FIDO2 PIN, entered at the login prompt, verified on the key
(never sent to authbox). It cannot be recovered, only checked or reset:

```bash
ykman fido info               # shows whether a PIN is set
ykman fido access change-pin  # set/change the PIN
```

#### Switching between touch-only and PIN

Switching modes changes **both** the credential and the PAM config, so both
playbooks are involved:

1. Set/verify the PIN on the key (`ykman`) if moving to PIN mode.
2. Re-enroll the key with the matching flags (with or without `-N`) and paste
   into the FIDO2 page. Revoke the old credential first.
3. Run `sync-fido2-mappings.yml` to push the new credential to
   `/etc/u2f_mappings`.
4. Edit `ansible/files/pam-u2f-auth` to add or remove `pinverification=1`, then
   run `enroll-host.yml` to redeploy the PAM line to the hosts.

Steps 3 and 4 must both happen; updating only one leaves the credential and PAM
line mismatched.

#### When to re-run the playbooks

- Re-run `sync-fido2-mappings.yml` after any enrollment, revocation, or
  re-enrollment, so `/etc/u2f_mappings` reflects the current credentials.
- Re-run `enroll-host.yml` after changing the PAM config (`pam-u2f-auth`,
  including adding/removing `pinverification=1`), or when enrolling a new host.

#### Debugging

Add `debug` to the PAM line and test the stack without logging out:

```bash
pamtester login someuser authenticate
```

Read the pam_u2f debug output (Debian minimal installs use the journal, not
`/var/log/auth.log`):

```bash
journalctl -b | grep -i u2f
```

Common failures seen in the debug output:

- **`Key not found in authenticator`** with `origin ... pam://<hostname>` - the
  origin does not match. The credential was enrolled under a different origin
  than the PAM line uses. Re-enroll with `-o pam://authbox -i pam://authbox` and
  set matching `origin=`/`appid=` on the PAM line.
- **Double colon in `/etc/u2f_mappings`** (`uid::keyhandle`) - the credential
  was stored with the leading colon from `pamu2fcfg -n`. Requires the fixed
  server build; re-enroll and re-sync.
- **`Unsupported options, skipping authenticator`** - the PAM line has
  `pinverification=1` but the credential is touch-only. Remove
  `pinverification=1` or re-enroll the key with a PIN.
- **Password prompt after a valid key** - the auth stack fell through because
  the key step did not succeed (one of the above), or the PAM rule is not
  `[success=done ...]`.

Test PAM changes on a second console (Ctrl+Alt+F3) with a root session held
open. The key path has no password fallback, so a broken config can lock the
console.

## Cert Expiration and Offboarding Automation

### The problem: certs can't be revoked, and sessions outlive the account

Authbox SSH access is certificate-based. When a user signs their key, the CA
issues a short-lived certificate (default TTL, see `SSH_CERT_TTL`). An OpenSSH
certificate is a self-contained, signed token: once issued, `sshd` accepts it
until it expires. There is no built-in revocation channel that authbox can push
to a fleet of hosts, so the standard offboarding actions do **not** immediately
stop access:

- **Removing the user from LDAP** stops NSS from resolving the account and
  blocks *future* logins that depend on the directory, but it does nothing to a
  certificate already in the user's possession, and nothing to a session already
  open.
- **Invalidating / "revoking" the cert in authbox** removes it from authbox's
  own records, but the signed cert on the user's laptop is still cryptographically
  valid until its TTL expires. `sshd` on each host has no way to know it was
  revoked.
- **Either action leaves active sessions untouched.** A user who is already
  logged in (SSH or console) keeps their shell, and any child processes keep
  running, regardless of what changes in the directory or in authbox.

Two optional mechanisms close these gaps. They are complementary: one blocks
*new* logins faster than TTL expiry, the other evicts *existing* sessions. Both
are off by default and enabled per-host via `enroll-host.yml` variables.

Because both rely on cron jobs and helper scripts that live on the login hosts,
enabling them requires configuration in two places: a small amount on the
**authbox server** (an endpoint plus, for revocation, a service account), and
per-host setup on **every client the user can log in to or SSH into**. A host
that is never enrolled with these settings will keep honoring valid certs and
keep active sessions alive.

### Certificate revocation (valid-serials allowlist)

Normally a signed cert is valid until it expires. This mechanism lets you revoke
a cert immediately: the host only accepts certs whose serials appear in a locally
cached allowlist fetched from authbox.

**How it works:**

- Authbox serves `GET /api/v1/ssh/valid-serials`, a bearer-token-authenticated
  endpoint that returns a plain-text list of `serial:principal` lines for every
  non-expired cert.
- On each host, `authbox-cert-cache-refresh.sh` runs on a cron. It obtains a
  bearer token by exchanging service account credentials, then uses the token to
  fetch the valid serials list and writes it to `/var/cache/authbox/valid-certs`.
- `sshd` is configured with an `AuthorizedPrincipalsCommand` pointing at
  `authbox-cert-check.sh`, which emits the principal only if the cert's serial
  is in the cache. Revoke a cert in authbox and it drops off the list; the next
  refresh removes it locally and further logins with that cert are denied.

**Enable in `enroll-host.yml`:**

```yaml
ssh_enforce_cert_validation: true
ssh_cert_cache_interval: 5   # cron refresh interval, minutes (integer 1-59)
```

**Revocation delay:** a revoked cert keeps working until the next cache refresh,
so worst case is one `ssh_cert_cache_interval`.

**Fail-closed:** if the cache file is missing, `authbox-cert-check.sh` emits no
principal, so login is denied. Note the current script trusts a *stale* cache
(if a refresh fails but an old file exists, its entries are still honored), so a
long-dead refresh could keep honoring an already-revoked cert.

### Service account for the cache refresh

`authbox-cert-cache-refresh.sh` authenticates to `GET /api/v1/ssh/valid-serials`
using OAuth2 service account credentials. Set it up:

**On the authbox server:**

1. Create a service account via the web UI (Admin role required).
2. Assign it the **viewer** role (required to call `valid-serials`).
3. Capture the one-time `client_id` and `client_secret` shown at creation.
4. Share these with your Ansible controller operator (next step).

Note: `HasRole` is not hierarchical except for admin. An `operator` token does
**not** satisfy a viewer check, so grant the account `viewer` specifically
(not `operator`). Use `admin` role only if the service account needs broader
platform access.

**On your Ansible controller:**

Pass the credentials as environment variables when running `enroll-host.yml`:

```bash
export CERT_REFRESH_CLIENT_ID="<client_id_from_ui>"
export CERT_REFRESH_CLIENT_SECRET="<client_secret_from_ui>"
ansible-playbook -i inventory.ini ansible/playbooks/enroll-host.yml
```

The playbook deploys the credentials securely to `/etc/secrets/authbox/authbox-cert-cache-refresh` on each host (mode 0640, readable only by root). The refresh script sources this file at runtime and exchanges the credentials for a bearer token on each cron tick.

**On each enrolled host:**

After Ansible runs:
- Credentials are stored at `/etc/secrets/authbox/authbox-cert-cache-refresh` (root-only, mode 0640).
- The `authbox-cert-cache-refresh.sh` script (deployed to `/usr/local/bin/`) runs on the configured interval via cron.
- Each run: obtains a token, fetches valid serials, and updates the local cache.
- Cron logs any errors to syslog (e.g., if credentials are invalid or authbox is unreachable).

**Credential rotation:**

To rotate credentials, create a new service account in the UI, update the Ansible controller environment variables, and re-run the playbook on affected hosts. Old credentials stop working immediately (the old service account can be deleted).

### Session termination for disabled users

Revocation blocks new logins but does not kick out active sessions. This is the
mechanism that solves the "user is disabled/removed but their shell is still
open" problem described in the problem section above. It applies to any local
session (SSH or console), not just SSH, because it acts on running processes
rather than on the login path.

**What it accomplishes:** a user who is disabled in authbox has their active
sessions terminated automatically within roughly one check interval, without an
operator having to hunt down and `kill` processes by hand on each host.

**How it works:**

- Disabling a user in authbox sets their login shell to `/sbin/nologin` in LDAP
  (and revokes their FIDO2 credentials).
- On each enrolled host, `authbox-session-check.sh` runs on a cron. It lists
  logged-in users (`who`), looks up each one's shell via NSS (which resolves
  through nslcd to LDAP), and `pkill`s any user whose shell is now
  `/sbin/nologin`.
- On the next tick after the disable propagates, the user's processes are
  killed, ending their sessions.

**Configure on the authbox server:** nothing specific to this mechanism. It
relies only on the existing disable action setting `/sbin/nologin`, which is
already part of the platform. The hosts read the shell over the normal LDAP/NSS
path configured during enrollment.

**Configure on every client the user can log in to / SSH into** (via
`enroll-host.yml`):

```yaml
ssh_kill_disabled_sessions: true
ssh_session_check_interval: 1   # cron check interval, minutes (integer 1-59)
```

This deploys `authbox-session-check.sh` to `/usr/local/bin/` and installs the
"authbox session check" cron job. Hosts that skip this stay vulnerable: a
disabled user's existing session keeps running there.

**Kill delay:** a disabled user's sessions persist until the next check plus the
nslcd cache TTL (the host must first see the updated `/sbin/nologin` shell via
LDAP).

### What to configure where (summary)

| Scope | Certificate revocation | Session termination |
| --- | --- | --- |
| **Authbox server** | Serves `GET /api/v1/ssh/valid-serials` (viewer-role bearer token required); provision a viewer-role service account | Nothing extra (disable action already sets `/sbin/nologin`) |
| **Every login/SSH client** | `ssh_enforce_cert_validation: true` (+ `ssh_cert_cache_interval`) in `enroll-host.yml`; provide service account credentials to Ansible via `CERT_REFRESH_CLIENT_ID` and `CERT_REFRESH_CLIENT_SECRET` env vars | `ssh_kill_disabled_sessions: true` (+ `ssh_session_check_interval`) in `enroll-host.yml` |

Both host-side settings must be applied to *every* host an authbox user can
reach. An unenrolled or partially-enrolled host silently keeps honoring valid
certs and keeps active sessions alive.

See [project.md](project.md) for full architecture documentation.
See [webstack.md](webstack.md) for web framework details.
See [webui.md](webui.md) for UI page specifications.

## Backup and Restore

### Export

From the web UI (Backup page), click "Export Now" to download a gzipped tar archive containing the LDAP directory, cn=config, and application state (FIDO2 credentials, service accounts, SSH certs).

For automation, use the API with a service account bearer token:

```bash
curl -sk -H "Authorization: Bearer $TOKEN" \
  https://host-local:8443/api/v1/config/export -o backup.tar.gz
```

### Restore via Web UI

1. Go to the Backup page
2. Upload the exported archive
3. Type "yesiagree" to confirm
4. The container will restart automatically and apply the restore

### Restore via CLI (manual)

If the web UI is unavailable, place LDIF files directly in the restore directory and restart the container:

```bash
# Extract the archive
mkdir /tmp/restore && tar xzf backup.tar.gz -C /tmp/restore

# Stage files for restore (path is on the Docker host volume)
docker cp /tmp/restore/directory.ldif authbox-primary:/data/live-restore/directory.ldif
docker cp /tmp/restore/config.ldif authbox-primary:/data/live-restore/config.ldif

# Restart - entrypoint will wipe existing data and apply the LDIFs
docker restart authbox-primary
```

The entrypoint checks for `/data/live-restore/` on startup. If found, it wipes the existing MDB and cn=config, runs `slapadd` with the staged files, removes the restore directory, then starts normally.

### CA Key Backup

The SSH CA private key is NOT included in standard exports. Back it up separately:

```bash
docker cp authbox-primary:/data/ca/ca_ed25519 ./ca_ed25519.backup
```

## Troubleshooting

### `redirect_uri_mismatch` on Google login

Google rejects the OIDC callback with "Error 400: redirect_uri_mismatch".

**Cause:** The redirect URI registered in Google Cloud Console doesn't match what Authbox sends. Authbox derives the redirect URI from `TLS_DOMAIN`:

```
https://<TLS_DOMAIN>:8443/auth/callback
```

**Fix:**
1. In Google Cloud Console, go to APIs and Services, then Credentials
2. Edit your OAuth 2.0 Client ID
3. Add `https://your-domain:8443/auth/callback` to Authorized redirect URIs
4. Wait 5-30 minutes for Google to propagate the change

**Note:** Newly added redirect URIs can take up to 30 minutes to become active. If the URI is correct but login still fails, wait and retry.

### `invalid state` after OIDC callback

**Cause:** The `oauth_state` cookie was set on a different hostname than the callback arrived on. This happens when you access Authbox via one hostname (e.g., `localhost`) but the callback redirects to another (e.g., your domain).

**Fix:** Access Authbox using the same hostname as `TLS_DOMAIN`. Don't mix `localhost` and your domain in the same session.

### `user not found in directory`

**Cause:** OIDC login succeeded but the user doesn't exist in LDAP. Users must be provisioned before they can log in.

**Fix:** Ensure `INITIAL_ADMIN_EMAIL` matches the Google/Entra email you're logging in with. On first boot, this user is created automatically. Additional users must be created via the web UI or API.
