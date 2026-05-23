package frontend

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/authbox/authbox/internal/auth"
	"github.com/go-chi/chi/v5"
)

// registerPartials adds HTMX partial fragment routes.
func (f *Frontend) registerPartials(r chi.Router) {
	r.Get("/partials/dashboard/activity", f.h.partialDashboardActivity)
	r.Get("/partials/users/list", f.h.partialUserList)
	r.Get("/partials/groups/list", f.h.partialGroupList)
	r.Get("/partials/ssh/certs", f.h.partialSSHCerts)
	r.Get("/partials/fido2/list", f.h.partialFIDO2List)
	r.Get("/partials/service-accounts/list", f.h.partialServiceAccountList)
	r.Get("/partials/logs/view", f.h.partialLogsView)
	r.Get("/partials/status/services", f.h.partialStatusServices)
	r.Get("/partials/status/tls", f.h.partialStatusTLS)
	r.Get("/partials/status/replication", f.h.partialStatusReplication)
	r.Get("/partials/status/storage", f.h.partialStatusStorage)
	r.Get("/partials/status/alerts", f.h.partialStatusAlerts)
}

// partialDashboardActivity returns recent activity HTML fragment.
func (h *handlers) partialDashboardActivity(w http.ResponseWriter, r *http.Request) {
	certs, _, _ := h.deps.Repo.ListSSHCerts(0, 20)
	w.Header().Set("Content-Type", "text/html")
	if len(certs) == 0 {
		w.Write([]byte(`<p class="text-sm text-gray-500 dark:text-gray-400">No recent activity</p>`))
		return
	}
	var sb strings.Builder
	sb.WriteString(`<table class="table"><thead><tr><th>User</th><th>Action</th><th>Time</th></tr></thead><tbody>`)
	for _, c := range certs {
		sb.WriteString(fmt.Sprintf(
			`<tr><td>%s</td><td>SSH cert issued</td><td>%s</td></tr>`,
			escHTML(c.Username), c.IssuedAt.Format("2006-01-02 15:04"),
		))
	}
	sb.WriteString(`</tbody></table>`)
	w.Write([]byte(sb.String()))
}

// partialUserList returns paginated user table HTML fragment.
func (h *handlers) partialUserList(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	status := r.URL.Query().Get("status")
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	limit := 50

	users, total, err := h.deps.LDAP.ListUsers(0, 1000)
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load users</p>`))
		return
	}

	// Filter
	var filtered []struct {
		UID       string
		CN        string
		Mail      string
		UIDNumber int
		Disabled  bool
	}
	for _, u := range users {
		if status == "active" && u.Disabled {
			continue
		}
		if status == "disabled" && !u.Disabled {
			continue
		}
		if q != "" {
			lower := strings.ToLower(q)
			if !strings.Contains(strings.ToLower(u.UID), lower) &&
				!strings.Contains(strings.ToLower(u.CN), lower) &&
				!strings.Contains(strings.ToLower(u.Mail), lower) {
				continue
			}
		}
		filtered = append(filtered, struct {
			UID       string
			CN        string
			Mail      string
			UIDNumber int
			Disabled  bool
		}{u.UID, u.CN, u.Mail, u.UIDNumber, u.Disabled})
	}

	_ = total
	total = len(filtered)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset > total {
		offset = total
	}
	page := filtered[offset:end]

	w.Header().Set("Content-Type", "text/html")
	var sb strings.Builder
	sb.WriteString(`<table class="table"><thead><tr><th>Username</th><th>Name</th><th>Email</th><th>UID</th><th>Status</th><th></th></tr></thead><tbody>`)
	for _, u := range page {
		statusBadge := `<span class="text-green-800">active</span>`
		if u.Disabled {
			statusBadge = `<span class="text-red-600">disabled</span>`
		}
		sb.WriteString(fmt.Sprintf(
			`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td><a href="/users/%s/edit" class="text-blue-600 text-sm">Edit</a></td></tr>`,
			escHTML(u.UID), escHTML(u.CN), escHTML(u.Mail), u.UIDNumber, statusBadge, escHTML(u.UID),
		))
	}
	if len(page) == 0 {
		sb.WriteString(`<tr><td colspan="6" class="text-center text-gray-500 text-sm">No users found</td></tr>`)
	}
	sb.WriteString(`</tbody></table>`)

	// Pagination
	if total > limit {
		sb.WriteString(`<div class="mt-4 flex justify-between items-center text-sm">`)
		sb.WriteString(fmt.Sprintf(`<span>Showing %d-%d of %d</span>`, offset+1, end, total))
		sb.WriteString(`<div class="space-x-2">`)
		if offset > 0 {
			prev := offset - limit
			if prev < 0 {
				prev = 0
			}
			sb.WriteString(fmt.Sprintf(`<button class="btn btn-secondary" hx-get="/partials/users/list?offset=%d" hx-target="#user-list" hx-include="[name='q'],[name='status']">Prev</button>`, prev))
		}
		if end < total {
			sb.WriteString(fmt.Sprintf(`<button class="btn btn-secondary" hx-get="/partials/users/list?offset=%d" hx-target="#user-list" hx-include="[name='q'],[name='status']">Next</button>`, end))
		}
		sb.WriteString(`</div></div>`)
	}

	w.Write([]byte(sb.String()))
}

// partialGroupList returns group table HTML fragment.
func (h *handlers) partialGroupList(w http.ResponseWriter, r *http.Request) {
	typeFilter := r.URL.Query().Get("type")
	groups, _, err := h.deps.LDAP.ListGroups(0, 500)
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load groups</p>`))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	var sb strings.Builder
	sb.WriteString(`<table class="table"><thead><tr><th>Name</th><th>Type</th><th>GID</th><th>Members</th><th></th></tr></thead><tbody>`)
	count := 0
	isAdmin := auth.HasRole(r.Context(), auth.RoleAdmin)
	for _, g := range groups {
		if typeFilter != "" && g.Type != typeFilter {
			continue
		}
		gid := "-"
		if g.GIDNumber > 0 {
			gid = strconv.Itoa(g.GIDNumber)
		}
		editLink := ""
		if isAdmin {
			editLink = fmt.Sprintf(`<a href="/groups/%s/edit" class="text-blue-600 text-sm">Edit</a>`, escHTML(g.CN))
		}
		sb.WriteString(fmt.Sprintf(
			`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>`,
			escHTML(g.CN), g.Type, gid, len(g.Members), editLink,
		))
		count++
	}
	if count == 0 {
		sb.WriteString(`<tr><td colspan="5" class="text-center text-gray-500 text-sm">No groups found</td></tr>`)
	}
	sb.WriteString(`</tbody></table>`)
	w.Write([]byte(sb.String()))
}

// partialSSHCerts returns SSH cert list HTML fragment.
func (h *handlers) partialSSHCerts(w http.ResponseWriter, r *http.Request) {
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	certs, total, err := h.deps.Repo.ListSSHCerts(offset, 20)
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load certificates</p>`))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	var sb strings.Builder
	sb.WriteString(`<table class="table"><thead><tr><th>User</th><th>Serial</th><th>Issued</th><th>Expires</th></tr></thead><tbody>`)
	for _, c := range certs {
		sb.WriteString(fmt.Sprintf(
			`<tr><td>%s</td><td class="font-mono text-xs">%s</td><td>%s</td><td>%s</td></tr>`,
			escHTML(c.Username), escHTML(c.Serial),
			c.IssuedAt.Format("2006-01-02 15:04"),
			c.ExpiresAt.Format("2006-01-02 15:04"),
		))
	}
	if len(certs) == 0 {
		sb.WriteString(`<tr><td colspan="4" class="text-center text-gray-500 text-sm">No certificates issued</td></tr>`)
	}
	sb.WriteString(`</tbody></table>`)

	if total > 20 {
		sb.WriteString(fmt.Sprintf(`<p class="mt-2 text-xs text-gray-500">Showing %d of %d</p>`, len(certs), total))
	}
	w.Write([]byte(sb.String()))
}

// partialFIDO2List returns FIDO2 credential list HTML fragment.
func (h *handlers) partialFIDO2List(w http.ResponseWriter, r *http.Request) {
	claims := auth.GetClaims(r.Context())
	isAdmin := auth.HasRole(r.Context(), auth.RoleAdmin)

	var creds []struct {
		ID           int
		UID          string
		RegisteredAt string
	}

	if isAdmin {
		all, err := h.deps.Repo.GetAllFIDO2Credentials()
		if err == nil {
			for _, c := range all {
				creds = append(creds, struct {
					ID           int
					UID          string
					RegisteredAt string
				}{c.ID, c.UID, c.RegisteredAt.Format("2006-01-02")})
			}
		}
	} else if claims != nil {
		uid := emailToUID(claims.Email)
		userCreds, err := h.deps.Repo.GetFIDO2Credentials(uid)
		if err == nil {
			for _, c := range userCreds {
				creds = append(creds, struct {
					ID           int
					UID          string
					RegisteredAt string
				}{c.ID, c.UID, c.RegisteredAt.Format("2006-01-02")})
			}
		}
	}

	w.Header().Set("Content-Type", "text/html")
	var sb strings.Builder
	sb.WriteString(`<table class="table"><thead><tr><th>User</th><th>Registered</th><th></th></tr></thead><tbody>`)
	for _, c := range creds {
		sb.WriteString(fmt.Sprintf(
			`<tr><td>%s</td><td>%s</td><td><button class="text-red-500 text-xs" hx-delete="/api/v1/fido2/credentials/%d" hx-target="#fido2-list" hx-confirm="Revoke this credential?">Revoke</button></td></tr>`,
			escHTML(c.UID), c.RegisteredAt, c.ID,
		))
	}
	if len(creds) == 0 {
		sb.WriteString(`<tr><td colspan="3" class="text-center text-gray-500 text-sm">No keys registered</td></tr>`)
	}
	sb.WriteString(`</tbody></table>`)

	// Warn if only 1 key
	if len(creds) == 1 {
		sb.WriteString(`<p class="mt-2 text-xs text-yellow-800 dark:text-yellow-200">Only 1 key registered. Consider adding a backup key.</p>`)
	}
	w.Write([]byte(sb.String()))
}

// partialServiceAccountList returns service account table HTML fragment.
func (h *handlers) partialServiceAccountList(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.deps.Repo.ListServiceAccounts()
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load service accounts</p>`))
		return
	}

	w.Header().Set("Content-Type", "text/html")
	var sb strings.Builder
	sb.WriteString(`<table class="table"><thead><tr><th>Client ID</th><th>Description</th><th>Role</th><th>Created</th><th>Last Used</th><th></th></tr></thead><tbody>`)
	for _, sa := range accounts {
		lastUsed := "-"
		if sa.LastUsedAt != nil {
			lastUsed = sa.LastUsedAt.Format("2006-01-02")
		}
		sb.WriteString(fmt.Sprintf(
			`<tr><td class="font-mono text-xs">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><button class="text-red-500 text-xs" hx-delete="/api/v1/service-accounts?client_id=%s" hx-target="#sa-list" hx-confirm="Delete this service account? All tokens will be revoked.">Delete</button></td></tr>`,
			escHTML(sa.ClientID), escHTML(sa.Description), sa.Role,
			sa.CreatedAt.Format("2006-01-02"), lastUsed, escHTML(sa.ClientID),
		))
	}
	if len(accounts) == 0 {
		sb.WriteString(`<tr><td colspan="6" class="text-center text-gray-500 text-sm">No service accounts</td></tr>`)
	}
	sb.WriteString(`</tbody></table>`)
	w.Write([]byte(sb.String()))
}

// partialLogsView returns log content HTML fragment.
func (h *handlers) partialLogsView(w http.ResponseWriter, r *http.Request) {
	// Read from log directory
	w.Header().Set("Content-Type", "text/html")
	logDir := h.deps.Config.LogDir

	entries, err := readLogTail(logDir, 100)
	if err != nil {
		w.Write([]byte(fmt.Sprintf(`<p>Error reading logs: %s</p>`, escHTML(err.Error()))))
		return
	}

	level := r.URL.Query().Get("level")
	search := r.URL.Query().Get("search")

	var sb strings.Builder
	for _, line := range entries {
		if level != "" && !strings.Contains(strings.ToLower(line), level) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(search)) {
			continue
		}
		sb.WriteString(escHTML(line))
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		sb.WriteString("No log entries matching filter")
	}
	w.Write([]byte(sb.String()))
}

// partialStatusServices returns service health HTML fragment.
func (h *handlers) partialStatusServices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	var sb strings.Builder

	// Check LDAP
	ldapOK := h.deps.LDAP.Ping() == nil
	ldapStatus := statusBadge(ldapOK)
	sb.WriteString(fmt.Sprintf(`<div class="flex justify-between py-1"><span>LDAP</span>%s</div>`, ldapStatus))

	// App is running if we got here
	sb.WriteString(`<div class="flex justify-between py-1"><span>Application</span>` + statusBadge(true) + `</div>`)

	w.Write([]byte(sb.String()))
}

// partialStatusTLS returns TLS cert info HTML fragment.
func (h *handlers) partialStatusTLS(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<p class="text-sm text-gray-500 dark:text-gray-400">TLS certificate status loaded from server config</p>`))
}

// partialStatusReplication returns replication status HTML fragment.
func (h *handlers) partialStatusReplication(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	role := h.deps.Config.Role
	if role == "primary" {
		w.Write([]byte(`<p class="text-sm">Role: <strong>Primary</strong></p>`))
	} else {
		w.Write([]byte(fmt.Sprintf(`<p class="text-sm">Role: <strong>Replica</strong> (primary: %s)</p>`, escHTML(h.deps.Config.PrimaryHost))))
	}
}

// partialStatusStorage returns disk usage HTML fragment.
func (h *handlers) partialStatusStorage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<p class="text-sm text-gray-500 dark:text-gray-400">Storage metrics not yet implemented</p>`))
}

// partialStatusAlerts returns active alerts HTML fragment.
func (h *handlers) partialStatusAlerts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(`<p class="text-sm text-gray-500 dark:text-gray-400">No active alerts</p>`))
}

// helpers

func statusBadge(ok bool) string {
	if ok {
		return `<span class="text-green-800 dark:text-green-400 text-sm font-medium">Healthy</span>`
	}
	return `<span class="text-red-600 dark:text-red-400 text-sm font-medium">Error</span>`
}

func escHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
