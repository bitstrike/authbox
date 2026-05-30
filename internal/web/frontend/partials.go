// partials.go contains HTMX partial fragment handlers that return HTML table
// snippets for dynamic page sections. Each partial fetches data, applies
// filtering/sorting/pagination using TableRenderer, and writes the HTML
// directly to the response. These are loaded via hx-get into .table-container
// elements on the parent pages.
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
	cfg := TableConfig{
		Columns: []Column{
			{Key: "username", Label: "User", Sortable: true},
			{Key: "action", Label: "Action", Sortable: false},
			{Key: "issued_at", Label: "Time", Sortable: true},
		},
		PartialURL: "/partials/dashboard/activity",
		Filterable: true,
	}

	state := ParseTableState(r, "issued_at")
	certs, total, _ := h.deps.Repo.ListSSHCertsSorted(state.Offset, state.Limit, state.Sort, state.Order)

	// Filter
	q := strings.ToLower(state.Query)
	type activityRow struct {
		Username string
		Action   string
		Time     string
	}
	var filtered []activityRow
	for _, c := range certs {
		row := activityRow{c.Username, "SSH cert issued", c.IssuedAt.Format("2006-01-02 15:04")}
		if q != "" && !strings.Contains(strings.ToLower(row.Username), q) && !strings.Contains(row.Time, q) {
			continue
		}
		filtered = append(filtered, row)
	}

	if q != "" {
		total = len(filtered)
	}
	state.Total = total

	w.Header().Set("Content-Type", "text/html")
	tr := NewTableRenderer(w, cfg, state)
	tr.RenderHeader()

	if len(filtered) == 0 {
		tr.RenderEmpty("No recent activity")
	} else {
		for _, row := range filtered {
			fmt.Fprintf(w, `<tr><td>%s</td><td>%s</td><td>%s</td></tr>`,
				escHTML(row.Username), row.Action, row.Time)
		}
	}

	tr.RenderFooter()
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

	// Build emoji lookup from employee types
	employeeTypes, _ := h.deps.Repo.ListEmployeeTypes()
	emojiMap := make(map[string]string)
	for _, et := range employeeTypes {
		emojiMap[et.Value] = et.Emoji
	}

	// Filter
	type userRow struct {
		UID          string
		CN           string
		Mail         string
		UIDNumber    int
		Disabled     bool
		EmployeeType string
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
		filtered = append(filtered, userRow{u.UID, u.CN, u.Mail, u.UIDNumber, u.Disabled, u.EmployeeType})
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
			typeBadge := ""
			if emoji, ok := emojiMap[u.EmployeeType]; ok && emoji != "" {
				typeBadge = emoji + " "
			}
			fmt.Fprintf(w,
				`<tr><td>%s%s</td><td>%s</td><td>%s</td><td>%d</td><td>%s</td><td><a href="/users/%s/edit" class="text-blue-600 text-sm">Edit</a></td></tr>`,
				typeBadge, escHTML(u.UID), escHTML(u.CN), escHTML(u.Mail), u.UIDNumber, statusBadge, escHTML(u.UID),
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
	w.Header().Set("Content-Type", "text/html")
	logDir := h.deps.Config.LogDir

	// Read full log file (not just tail) so filters work across all entries
	entries, err := readLogFull(logDir)
	if err != nil {
		w.Write([]byte(fmt.Sprintf(`<p>Error reading logs: %s</p>`, escHTML(err.Error()))))
		return
	}

	level := r.URL.Query().Get("level")
	search := r.URL.Query().Get("search")

	// Filter
	var filtered []string
	for _, line := range entries {
		if level != "" && !strings.Contains(strings.ToLower(line), "["+level+"]") {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(search)) {
			continue
		}
		filtered = append(filtered, line)
	}

	// Show last 500 matching lines
	const maxDisplay = 500
	if len(filtered) > maxDisplay {
		filtered = filtered[len(filtered)-maxDisplay:]
	}

	var sb strings.Builder
	if len(filtered) == 0 {
		sb.WriteString("No log entries matching filter")
	} else {
		for _, line := range filtered {
			sb.WriteString(escHTML(line))
			sb.WriteString("\n")
		}
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

// Settings partial handlers

func (h *handlers) partialSettingsOIDC(w http.ResponseWriter, r *http.Request) {
	data := struct {
		OIDCIssuer           string
		OIDCClientID         string
		OIDCSecretConfigured bool
	}{
		OIDCIssuer:           h.deps.Config.OIDCIssuerURL,
		OIDCClientID:         h.deps.Config.OIDCClientID,
		OIDCSecretConfigured: h.deps.Config.OIDCClientSecret != "",
	}
	h.renderer.renderPartial(w, "settings_oidc", data)
}

func (h *handlers) partialSettingsSession(w http.ResponseWriter, r *http.Request) {
	data := struct {
		SessionTimeout int
	}{
		SessionTimeout: 30,
	}
	h.renderer.renderPartial(w, "settings_session", data)
}

func (h *handlers) partialSettingsUIDRange(w http.ResponseWriter, r *http.Request) {
	data := struct {
		UIDRangeStart string
		UIDRangeEnd   string
	}{
		UIDRangeStart: h.deps.Config.UIDRangeStart,
		UIDRangeEnd:   h.deps.Config.UIDRangeEnd,
	}
	h.renderer.renderPartial(w, "settings_uid_range", data)
}

func (h *handlers) partialSettingsSSHCA(w http.ResponseWriter, r *http.Request) {
	data := struct {
		CAPublicKey string
		SSHCertTTL  string
	}{
		CAPublicKey: h.deps.CA.PublicKeyString(),
		SSHCertTTL:  h.deps.Config.SSHCertTTL,
	}
	h.renderer.renderPartial(w, "settings_ssh_ca", data)
}

func (h *handlers) partialSettingsLDAP(w http.ResponseWriter, r *http.Request) {
	data := struct {
		LDAPBaseDN string
	}{
		LDAPBaseDN: h.deps.Config.LDAPBaseDN,
	}
	h.renderer.renderPartial(w, "settings_ldap", data)
}

func (h *handlers) partialSettingsLogging(w http.ResponseWriter, r *http.Request) {
	data := struct {
		LogLevel     string
		LogRetention int
	}{
		LogLevel:     h.deps.Config.LogLevel,
		LogRetention: 90,
	}
	h.renderer.renderPartial(w, "settings_logging", data)
}

func (h *handlers) partialSettingsEmployeeTypes(w http.ResponseWriter, r *http.Request) {
	h.renderer.renderPartial(w, "settings_employee_types", nil)
}

func (h *handlers) partialSettingsEmployeeTypesList(w http.ResponseWriter, r *http.Request) {
	types, _ := h.deps.Repo.ListEmployeeTypes()
	data := struct {
		Types []db.EmployeeType
	}{Types: types}
	h.renderer.renderPartial(w, "settings_employee_types_list", data)
}

func (h *handlers) actionCreateEmployeeType(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	value := r.FormValue("value")
	label := r.FormValue("label")
	emoji := r.FormValue("emoji")

	if value == "" || label == "" {
		h.partialSettingsEmployeeTypesList(w, r)
		return
	}

	// Determine next sort order
	types, _ := h.deps.Repo.ListEmployeeTypes()
	nextOrder := len(types) + 1

	h.deps.Repo.CreateEmployeeType(&db.EmployeeType{
		Value:     value,
		Label:     label,
		Emoji:     emoji,
		SortOrder: nextOrder,
	})
	h.partialSettingsEmployeeTypesList(w, r)
}

func (h *handlers) actionDeleteEmployeeType(w http.ResponseWriter, r *http.Request) {
	value := chi.URLParam(r, "value")
	h.deps.Repo.DeleteEmployeeType(value)
	h.partialSettingsEmployeeTypesList(w, r)
}

// Backup partial handlers

func (h *handlers) partialBackupExport(w http.ResponseWriter, r *http.Request) {
	h.renderer.renderPartial(w, "backup_export", nil)
}

func (h *handlers) partialBackupImport(w http.ResponseWriter, r *http.Request) {
	h.renderer.renderPartial(w, "backup_import", nil)
}

func (h *handlers) partialBackupSchedule(w http.ResponseWriter, r *http.Request) {
	data := struct {
		BackupEnabled   bool
		BackupTime      string
		BackupRetention int
	}{
		BackupTime:      "02:00",
		BackupRetention: 30,
	}
	h.renderer.renderPartial(w, "backup_schedule", data)
}

func (h *handlers) partialBackupCAKey(w http.ResponseWriter, r *http.Request) {
	data := struct {
		CAFingerprint string
	}{
		CAFingerprint: h.deps.CA.Fingerprint(),
	}
	h.renderer.renderPartial(w, "backup_ca_key", data)
}
