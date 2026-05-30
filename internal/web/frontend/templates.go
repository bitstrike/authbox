// templates.go manages Go html/template loading and rendering. Defines the
// PageData struct passed to every template (title, user email, roles, content)
// and the templateRenderer which pre-parses all page templates with the shared
// layout on startup. Also provides pageDataFromRequest to build PageData from
// the current request context.
package frontend

import (
	"embed"
	"html/template"
	"io"
	"net/http"
	"sync"

	"github.com/authbox/authbox/internal/auth"
)

//go:embed templates/*.html
var templateFS embed.FS

// PageData holds common data passed to every template.
type PageData struct {
	Title       string
	CurrentPath string
	Email       string
	Roles       []auth.Role
	IsAdmin     bool
	IsOperator  bool
	IsViewer    bool
	Flash       string
	Content     any
}

type templateRenderer struct {
	mu    sync.RWMutex
	tmpls map[string]*template.Template
	funcs template.FuncMap
}

func newRenderer() *templateRenderer {
	funcs := template.FuncMap{
		"hasRole": func(roles []auth.Role, role string) bool {
			for _, r := range roles {
				if string(r) == role || r == auth.RoleAdmin {
					return true
				}
			}
			return false
		},
	}
	r := &templateRenderer{
		tmpls: make(map[string]*template.Template),
		funcs: funcs,
	}
	r.loadAll()
	return r
}

func (tr *templateRenderer) loadAll() {
	pages := []string{
		"login", "dashboard", "users", "user_form", "user_import",
		"groups", "group_form", "ssh", "fido2", "service_accounts",
		"logs", "status", "settings", "backup",
	}
	for _, name := range pages {
		t := template.New("layout.html").Funcs(tr.funcs)
		t = template.Must(t.ParseFS(templateFS, "templates/layout.html", "templates/"+name+".html"))
		tr.tmpls[name] = t
	}
	// Login has no layout
	t := template.New("login_standalone.html").Funcs(tr.funcs)
	t = template.Must(t.ParseFS(templateFS, "templates/login_standalone.html"))
	tr.tmpls["login_standalone"] = t

	// Settings partials (no layout, rendered as fragments)
	settingsPartials := []string{
		"settings_oidc", "settings_session", "settings_uid_range",
		"settings_ssh_ca", "settings_ldap", "settings_logging",
		"settings_employee_types",
	}
	for _, name := range settingsPartials {
		t := template.New(name + ".html").Funcs(tr.funcs)
		t = template.Must(t.ParseFS(templateFS, "templates/"+name+".html"))
		tr.tmpls[name] = t
	}
}

func (tr *templateRenderer) render(w io.Writer, name string, data PageData) error {
	tr.mu.RLock()
	t, ok := tr.tmpls[name]
	tr.mu.RUnlock()
	if !ok {
		return nil
	}
	return t.Execute(w, data)
}

func (tr *templateRenderer) renderPage(w http.ResponseWriter, name string, data PageData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tr.render(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (tr *templateRenderer) renderPartial(w http.ResponseWriter, name string, data any) {
	tr.mu.RLock()
	t, ok := tr.tmpls[name]
	tr.mu.RUnlock()
	if !ok {
		http.Error(w, "template not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, name, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func pageDataFromRequest(r *http.Request, title string, content any) PageData {
	claims := auth.GetClaims(r.Context())
	roles := auth.GetRoles(r.Context())
	email := ""
	if claims != nil {
		email = claims.Email
	}
	return PageData{
		Title:       title,
		CurrentPath: r.URL.Path,
		Email:       email,
		Roles:       roles,
		IsAdmin:     auth.HasRole(r.Context(), auth.RoleAdmin),
		IsOperator:  auth.HasRole(r.Context(), auth.RoleOperator),
		IsViewer:    auth.HasRole(r.Context(), auth.RoleViewer),
		Content:     content,
	}
}
