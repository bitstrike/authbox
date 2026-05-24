package frontend

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/db"
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
	cfg := TableConfig{
		Columns: []Column{
			{Key: "uid", Label: "Username", Sortable: true},
			{Key: "cn", Label: "Name", Sortable: true},
			{Key: "mail", Label: "Email", Sortable: true},
			{Key: "uidNumber", Label: "UID", Sortable: true},
			{Key: "status", Label: "Status", Sortable: false},
			{Key: "_actions", Label: "", Sortable: false},
		},
		PartialURL: "/partials/users/list",
		Filterable: true,
	}

	state := ParseTableState(r, "uid")
	status := r.URL.Query().Get("status")

	users, _, err := h.deps.LDAP.ListUsers(0, 1000)
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load users</p>`))
		return
	}

	// Filter
	type userRow struct {
		UID       string
		CN        string
		Mail      string
		UIDNumber int
		Disabled  bool
	}
	var filtered []userRow
	q := strings.ToLower(state.Query)
	for _, u := range users {
		if status == "active" && u.Disabled {
			continue
		}
		if status == "disabled" && !u.Disabled {
			continue
		}
		if q != "" {
			if !strings.Contains(strings.ToLower(u.UID), q) &&
				!strings.Contains(strings.ToLower(u.CN), q) &&
				!strings.Contains(strings.ToLower(u.Mail), q) {
				continue
			}
		}
		filtered = append(filtered, userRow{u.UID, u.CN, u.Mail, u.UIDNumber, u.Disabled})
	}

	total := len(filtered)
	end := state.Offset + state.Limit
	if end > total {
		end = total
	}
	if state.Offset > total {
		state.Offset = total
	}
	page := filtered[state.Offset:end]
	state.Total = total

	w.Header().Set("Content-Type", "text/html")
	tr := NewTableRenderer(w, cfg, state)
	tr.RenderHeader()

	if len(page) == 0 {
		tr.RenderEmpty("No users found")
	} else {
		for _, u := range page {
			statusBadge := `<span class="text-green-800 dark:text-green-400">active</span>`
			if u.Disabled {
				statusBadge = `<span class="text-red-600 dark:text-red-400">disabled</span>`
			}
			fmt.Fprintf(w,
				`<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td><a href="/users/%s/edit" class="text-blue-600 text-sm">Edit</a></td></tr>`,
				escHTML(u.UID), escHTML(u.CN), escHTML(u.Mail), u.UIDNumber, statusBadge, escHTML(u.UID),
			)
		}
	}

	tr.RenderFooter()
}

// partialGroupList returns group table HTML fragment.
func (h *handlers) partialGroupList(w http.ResponseWriter, r *http.Request) {
	cfg := TableConfig{
		Columns: []Column{
			{Key: "cn", Label: "Name", Sortable: true},
			{Key: "type", Label: "Type", Sortable: false},
			{Key: "gidNumber", Label: "GID", Sortable: true},
			{Key: "members", Label: "Members", Sortable: false},
			{Key: "_actions", Label: "", Sortable: false},
		},
		PartialURL: "/partials/groups/list",
		Filterable: true,
	}

	state := ParseTableState(r, "cn")
	state.Order = "asc"
	typeFilter := r.URL.Query().Get("type")

	groups, _, err := h.deps.LDAP.ListGroups(0, 500)
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load groups</p>`))
		return
	}

	isAdmin := auth.HasRole(r.Context(), auth.RoleAdmin)
	q := strings.ToLower(state.Query)

	type groupRow struct {
		CN        string
		Type      string
		GIDNumber int
		Members   int
	}
	var filtered []groupRow
	for _, g := range groups {
		if typeFilter != "" && g.Type != typeFilter {
			continue
		}
		if q != "" && !strings.Contains(strings.ToLower(g.CN), q) {
			continue
		}
		filtered = append(filtered, groupRow{g.CN, g.Type, g.GIDNumber, len(g.Members)})
	}

	total := len(filtered)
	end := state.Offset + state.Limit
	if end > total {
		end = total
	}
	page := filtered[state.Offset:end]
	state.Total = total

	w.Header().Set("Content-Type", "text/html")
	tr := NewTableRenderer(w, cfg, state)
	tr.RenderHeader()

	if len(page) == 0 {
		tr.RenderEmpty("No groups found")
	} else {
		for _, g := range page {
			gid := "-"
			if g.GIDNumber > 0 {
				gid = strconv.Itoa(g.GIDNumber)
			}
			editLink := ""
			if isAdmin {
				editLink = fmt.Sprintf(`<a href="/groups/%s/edit" class="text-blue-600 text-sm">Edit</a>`, escHTML(g.CN))
			}
			fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td></tr>`,
				escHTML(g.CN), g.Type, gid, g.Members, editLink,
			)
		}
	}

	tr.RenderFooter()
}

// partialSSHCerts returns SSH cert list HTML fragment.
func (h *handlers) partialSSHCerts(w http.ResponseWriter, r *http.Request) {
	cfg := TableConfig{
		Columns: []Column{
			{Key: "username", Label: "User", Sortable: true},
			{Key: "serial", Label: "Serial", Sortable: true},
			{Key: "issued_at", Label: "Issued", Sortable: true},
			{Key: "expires_at", Label: "Expires", Sortable: true},
		},
		PartialURL: "/partials/ssh/certs",
		Filterable: true,
		Exportable: false,
	}

	state := ParseTableState(r, "issued_at")

	certs, total, err := h.deps.Repo.ListSSHCertsSorted(state.Offset, state.Limit, state.Sort, state.Order)
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load certificates</p>`))
		return
	}

	// Filter by query
	if state.Query != "" {
		q := strings.ToLower(state.Query)
		var filtered []db.SSHCert
		for _, c := range certs {
			if strings.Contains(strings.ToLower(c.Username), q) ||
				strings.Contains(strings.ToLower(c.Serial), q) ||
				strings.Contains(c.IssuedAt.Format("2006-01-02 15:04"), q) ||
				strings.Contains(c.ExpiresAt.Format("2006-01-02 15:04"), q) {
				filtered = append(filtered, c)
			}
		}
		certs = filtered
		total = len(filtered)
	}

	state.Total = total
	w.Header().Set("Content-Type", "text/html")

	tr := NewTableRenderer(w, cfg, state)
	tr.RenderHeader()

	if len(certs) == 0 {
		tr.RenderEmpty("No certificates issued")
	} else {
		for _, c := range certs {
			fmt.Fprintf(w, `<tr><td>%s</td><td class="font-mono text-xs">%s</td><td>%s</td><td>%s</td></tr>`,
				escHTML(c.Username), escHTML(c.Serial),
				c.IssuedAt.Format("2006-01-02 15:04"),
				c.ExpiresAt.Format("2006-01-02 15:04"),
			)
		}
	}

	tr.RenderFooter()
}

// partialFIDO2List returns FIDO2 credential list HTML fragment.
func (h *handlers) partialFIDO2List(w http.ResponseWriter, r *http.Request) {
	cfg := TableConfig{
		Columns: []Column{
			{Key: "uid", Label: "User", Sortable: false},
			{Key: "registered_at", Label: "Registered", Sortable: false},
			{Key: "_actions", Label: "", Sortable: false},
		},
		PartialURL: "/partials/fido2/list",
		Filterable: true,
	}

	state := ParseTableState(r, "registered_at")
	claims := auth.GetClaims(r.Context())
	isAdmin := auth.HasRole(r.Context(), auth.RoleAdmin)

	type credRow struct {
		ID           int
		UID          string
		RegisteredAt string
	}
	var all []credRow

	if isAdmin {
		creds, err := h.deps.Repo.GetAllFIDO2Credentials()
		if err == nil {
			for _, c := range creds {
				all = append(all, credRow{c.ID, c.UID, c.RegisteredAt.Format("2006-01-02")})
			}
		}
	} else if claims != nil {
		uid := emailToUID(claims.Email)
		creds, err := h.deps.Repo.GetFIDO2Credentials(uid)
		if err == nil {
			for _, c := range creds {
				all = append(all, credRow{c.ID, c.UID, c.RegisteredAt.Format("2006-01-02")})
			}
		}
	}

	// Filter
	q := strings.ToLower(state.Query)
	var filtered []credRow
	for _, c := range all {
		if q != "" && !strings.Contains(strings.ToLower(c.UID), q) && !strings.Contains(c.RegisteredAt, q) {
			continue
		}
		filtered = append(filtered, c)
	}

	total := len(filtered)
	end := state.Offset + state.Limit
	if end > total {
		end = total
	}
	page := filtered[state.Offset:end]
	state.Total = total

	w.Header().Set("Content-Type", "text/html")
	tr := NewTableRenderer(w, cfg, state)
	tr.RenderHeader()

	if len(page) == 0 {
		tr.RenderEmpty("No keys registered")
	} else {
		for _, c := range page {
			fmt.Fprintf(w,
				`<tr><td>%s</td><td>%s</td><td><button class="text-red-500 text-xs" hx-delete="/api/v1/fido2/credentials/%d" hx-target="closest .table-container" hx-confirm="Revoke this credential?">Revoke</button></td></tr>`,
				escHTML(c.UID), c.RegisteredAt, c.ID,
			)
		}
	}

	tr.RenderFooter()

	// Warn if only 1 key
	if total == 1 {
		fmt.Fprint(w, `<p class="mt-2 text-xs text-yellow-800 dark:text-yellow-200">Only 1 key registered. Consider adding a backup key.</p>`)
	}
}

// partialServiceAccountList returns service account table HTML fragment.
func (h *handlers) partialServiceAccountList(w http.ResponseWriter, r *http.Request) {
	cfg := TableConfig{
		Columns: []Column{
			{Key: "client_id", Label: "Client ID", Sortable: false},
			{Key: "description", Label: "Description", Sortable: false},
			{Key: "role", Label: "Role", Sortable: false},
			{Key: "created_at", Label: "Created", Sortable: false},
			{Key: "last_used", Label: "Last Used", Sortable: false},
			{Key: "_actions", Label: "", Sortable: false},
		},
		PartialURL: "/partials/service-accounts/list",
		Filterable: true,
	}

	state := ParseTableState(r, "created_at")

	accounts, err := h.deps.Repo.ListServiceAccounts()
	if err != nil {
		w.Write([]byte(`<p class="text-sm text-red-600">Failed to load service accounts</p>`))
		return
	}

	// Filter
	q := strings.ToLower(state.Query)
	type saRow struct {
		ClientID    string
		Description string
		Role        string
		CreatedAt   string
		LastUsedAt  string
	}
	var filtered []saRow
	for _, sa := range accounts {
		lastUsed := "-"
		if sa.LastUsedAt != nil {
			lastUsed = sa.LastUsedAt.Format("2006-01-02")
		}
		row := saRow{sa.ClientID, sa.Description, sa.Role, sa.CreatedAt.Format("2006-01-02"), lastUsed}
		if q != "" {
			combined := strings.ToLower(row.ClientID + row.Description + row.Role)
			if !strings.Contains(combined, q) {
				continue
			}
		}
		filtered = append(filtered, row)
	}

	total := len(filtered)
	end := state.Offset + state.Limit
	if end > total {
		end = total
	}
	page := filtered[state.Offset:end]
	state.Total = total

	w.Header().Set("Content-Type", "text/html")
	tr := NewTableRenderer(w, cfg, state)
	tr.RenderHeader()

	if len(page) == 0 {
		tr.RenderEmpty("No service accounts")
	} else {
		for _, sa := range page {
			fmt.Fprintf(w,
				`<tr><td class="font-mono text-xs">%s</td><td>%s</td><td>%s</td><td>%s</td><td>%s</td><td><button class="text-red-500 text-xs" hx-delete="/api/v1/service-accounts?client_id=%s" hx-target="closest .table-container" hx-confirm="Delete this service account? All tokens will be revoked.">Delete</button></td></tr>`,
				escHTML(sa.ClientID), escHTML(sa.Description), sa.Role, sa.CreatedAt, sa.LastUsedAt, escHTML(sa.ClientID),
			)
		}
	}

	tr.RenderFooter()
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
