// handlers.go contains the GET handlers that render full HTML pages. Each
// handler fetches data from LDAP/SQLite, builds a content struct, and calls
// renderPage with the appropriate template name. The Deps struct holds all
// injected dependencies (LDAP client, CA, repository, config, sessions).
package frontend

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/backup"
	"github.com/authbox/authbox/internal/ca"
	"github.com/authbox/authbox/internal/config"
	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/ldap"
	"github.com/authbox/authbox/internal/logging"
	"github.com/go-chi/chi/v5"
)

// Deps holds dependencies injected into the frontend handlers.
type Deps struct {
	LDAP     *ldap.Client
	CA       *ca.CA
	Repo     *db.Repository
	Config   *config.Config
	Sessions *auth.SessionStore
	Roles    auth.RoleLookup
	Log      *logging.Logger
}

// handlers holds all page handler methods.
type handlers struct {
	deps        *Deps
	renderer    *templateRenderer
	signLimiter *rateLimiter
}

func newHandlers(deps *Deps) *handlers {
	return &handlers{
		deps:        deps,
		renderer:    newRenderer(),
		signLimiter: newRateLimiter(),
	}
}

// Dashboard
func (h *handlers) dashboard(w http.ResponseWriter, r *http.Request) {
	stats := h.gatherDashboardStats()
	data := pageDataFromRequest(r, "Dashboard", stats)
	h.renderer.renderPage(w, "dashboard", data)
}

// Users list
func (h *handlers) users(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "Users", nil)
	h.renderer.renderPage(w, "users", data)
}

// User create form
func (h *handlers) userNew(w http.ResponseWriter, r *http.Request) {
	content := struct {
		IsEdit bool
		Action string
		User   ldap.User
		Error  string
	}{
		IsEdit: false,
		Action: "/users",
		User:   ldap.User{LoginShell: "/bin/bash"},
	}
	data := pageDataFromRequest(r, "Create User", content)
	h.renderer.renderPage(w, "user_form", data)
}

// User edit form
func (h *handlers) userEdit(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	user, err := h.deps.LDAP.GetUser(uid)
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	content := struct {
		IsEdit bool
		Action string
		User   ldap.User
		Error  string
	}{
		IsEdit: true,
		Action: "/users/" + uid,
		User:   *user,
	}
	data := pageDataFromRequest(r, "Edit User", content)
	h.renderer.renderPage(w, "user_form", data)
}

// User import page
func (h *handlers) userImport(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "Bulk Import", nil)
	h.renderer.renderPage(w, "user_import", data)
}

// Groups list
func (h *handlers) groups(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "Groups", nil)
	h.renderer.renderPage(w, "groups", data)
}

// Group create form
func (h *handlers) groupNew(w http.ResponseWriter, r *http.Request) {
	content := struct {
		IsEdit  bool
		Action  string
		Group   ldap.Group
		Members []string
		Error   string
	}{
		IsEdit: false,
		Action: "/groups",
	}
	data := pageDataFromRequest(r, "Create Group", content)
	h.renderer.renderPage(w, "group_form", data)
}

// Group edit form
func (h *handlers) groupEdit(w http.ResponseWriter, r *http.Request) {
	cn := chi.URLParam(r, "cn")
	group, err := h.deps.LDAP.GetGroup(cn)
	if err != nil || group == nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	content := struct {
		IsEdit  bool
		Action  string
		Group   ldap.Group
		Members []string
		Error   string
	}{
		IsEdit:  true,
		Action:  "/groups/" + cn,
		Group:   *group,
		Members: group.Members,
	}
	data := pageDataFromRequest(r, "Edit Group", content)
	h.renderer.renderPage(w, "group_form", data)
}

// SSH certs page
func (h *handlers) ssh(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "SSH Certificates", nil)
	h.renderer.renderPage(w, "ssh", data)
}

// FIDO2 page
func (h *handlers) fido2(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "FIDO2 Keys", nil)
	h.renderer.renderPage(w, "fido2", data)
}

// Service accounts page
func (h *handlers) serviceAccounts(w http.ResponseWriter, r *http.Request) {
	content := struct {
		NewSecret   string
		NewClientID string
	}{}
	data := pageDataFromRequest(r, "Service Accounts", content)
	h.renderer.renderPage(w, "service_accounts", data)
}

// Logs page
func (h *handlers) logs(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "Logs", nil)
	h.renderer.renderPage(w, "logs", data)
}

// Status page
func (h *handlers) status(w http.ResponseWriter, r *http.Request) {
	data := pageDataFromRequest(r, "System Status", nil)
	h.renderer.renderPage(w, "status", data)
}

// Settings page
func (h *handlers) settings(w http.ResponseWriter, r *http.Request) {
	var buf strings.Builder
	sr := NewSidebarRenderer(&buf, SidebarConfig{
		PanelID:    "settings-panel",
		DefaultURL: "/settings/oidc",
		NavItems: []SidebarNavItem{
			{Label: "OIDC Provider", URL: "/settings/oidc"},
			{Label: "Session", URL: "/settings/session"},
			{Label: "UID/GID Range", URL: "/settings/uid-range"},
			{Label: "SSH CA", URL: "/settings/ssh-ca"},
			{Label: "LDAP", URL: "/settings/ldap"},
			{Label: "Logging", URL: "/settings/logging"},
			{Label: "Employee Types", URL: "/settings/employee-types"},
		},
	})
	sr.Render()
	data := pageDataFromRequest(r, "Settings", template.HTML(buf.String()))
	h.renderer.renderPage(w, "settings", data)
}

// Backup page
func (h *handlers) backup(w http.ResponseWriter, r *http.Request) {
	var buf strings.Builder
	sr := NewSidebarRenderer(&buf, SidebarConfig{
		PanelID:    "backup-panel",
		DefaultURL: "/backup/export-panel",
		NavItems: []SidebarNavItem{
			{Label: "Export", URL: "/backup/export-panel"},
			{Label: "Import", URL: "/backup/import-panel"},
			{Label: "Schedule", URL: "/backup/schedule"},
			{Label: "CA Key", URL: "/backup/ca-key"},
		},
	})
	sr.Render()
	data := pageDataFromRequest(r, "Backup", template.HTML(buf.String()))
	h.renderer.renderPage(w, "backup", data)
}

// Backup export (session-authenticated, streams archive to browser)
func (h *handlers) actionExportBackup(w http.ResponseWriter, r *http.Request) {
	ts := time.Now().Format("2006-01-02T150405")
	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=authbox-export-%s.tar.gz", ts))
	if err := backup.CreateExport(w, h.deps.Repo, "/usr/sbin/slapcat"); err != nil {
		http.Error(w, "export failed: "+err.Error(), http.StatusInternalServerError)
	}
}

// Backup import (session-authenticated, restores from uploaded archive)
func (h *handlers) actionImportBackup(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(50 << 20) // 50MB max

	confirm := r.FormValue("confirm")
	if confirm != "yesiagree" {
		h.renderBackupError(w, r, "Type \"yesiagree\" to confirm import")
		return
	}

	file, _, err := r.FormFile("archive")
	if err != nil {
		h.renderBackupError(w, r, "Archive file required")
		return
	}
	defer file.Close()

	claims := auth.GetClaims(r.Context())
	email := ""
	if claims != nil {
		email = claims.Email
	}
	h.deps.Log.Info("backup import started", "user", email)

	data, err := backup.ImportExport(file)
	if err != nil {
		h.deps.Log.Error("backup import failed: invalid archive", "user", email, "err", err)
		h.renderBackupError(w, r, "Invalid archive: "+err.Error())
		return
	}

	// Stage LDIF files for restore on next startup
	restoreDir := "/data/live-restore"
	if err := os.MkdirAll(restoreDir, 0750); err != nil {
		h.deps.Log.Error("backup import failed: create restore dir", "err", err)
		h.renderBackupError(w, r, "Failed to create restore directory: "+err.Error())
		return
	}

	if len(data.DirectoryLDIF) > 0 {
		if err := os.WriteFile(restoreDir+"/directory.ldif", data.DirectoryLDIF, 0640); err != nil {
			h.deps.Log.Error("backup import failed: write directory LDIF", "err", err)
			h.renderBackupError(w, r, "Failed to write directory LDIF: "+err.Error())
			return
		}
	}

	if len(data.ConfigLDIF) > 0 {
		if err := os.WriteFile(restoreDir+"/config.ldif", data.ConfigLDIF, 0640); err != nil {
			h.deps.Log.Error("backup import failed: write config LDIF", "err", err)
			h.renderBackupError(w, r, "Failed to write config LDIF: "+err.Error())
			return
		}
	}

	h.deps.Log.Info("backup import: LDIF files staged to /data/live-restore", "user", email)

	// Restore SQLite state immediately (independent of slapd)
	if err := backup.RestoreState(h.deps.Repo, &data.State); err != nil {
		h.deps.Log.Error("backup import failed: state restore", "err", err)
		h.renderBackupError(w, r, "State restore failed: "+err.Error())
		return
	}

	h.deps.Log.Info("backup import: sqlite state restored, triggering restart", "user", email)

	// Exit to trigger container restart; entrypoint will apply the staged LDIF
	go func() {
		time.Sleep(500 * time.Millisecond)
		h.deps.Log.Sync()
		os.Exit(0)
	}()

	// Respond before exit
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(`<html><body><h1>Import staged</h1><p>Container is restarting to apply LDAP restore. Reload this page in a few seconds.</p></body></html>`))
}

func (h *handlers) renderBackupError(w http.ResponseWriter, r *http.Request, errMsg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	fmt.Fprintf(w, `<div class="p-3 rounded bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-sm mb-4">%s</div>`, errMsg)
	fmt.Fprint(w, `<a href="/backup" class="btn btn-secondary">Back to Backup</a>`)
}
