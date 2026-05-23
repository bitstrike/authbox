package frontend

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/ldap"
	"github.com/go-chi/chi/v5"
)

// registerActions adds POST/PUT/DELETE form action routes.
func (f *Frontend) registerActions(r chi.Router) {
	// Users
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleOperator))
		r.Post("/users", f.h.actionCreateUser)
		r.Post("/users/{uid}", f.h.actionUpdateUser)
		r.Post("/users/{uid}/disable", f.h.actionDisableUser)
		r.Post("/users/import", f.h.actionImportUsers)
	})
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/users/{uid}/enable", f.h.actionEnableUser)
	})

	// Groups
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/groups", f.h.actionCreateGroup)
		r.Post("/groups/{cn}", f.h.actionUpdateGroup)
		r.Post("/groups/{cn}/delete", f.h.actionDeleteGroup)
		r.Post("/groups/{cn}/members", f.h.actionAddMember)
	})

	// SSH
	r.Post("/ssh/sign", f.h.actionSignSSH)

	// FIDO2
	r.Post("/fido2/register", f.h.actionRegisterFIDO2)

	// Service accounts (admin)
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/service-accounts", f.h.actionCreateServiceAccount)
	})
}

func (h *handlers) actionCreateUser(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	uidNum, _ := strconv.Atoi(r.FormValue("uidNumber"))
	gidNum, _ := strconv.Atoi(r.FormValue("gidNumber"))

	user := &ldap.User{
		UID:           r.FormValue("uid"),
		CN:            r.FormValue("givenName") + " " + r.FormValue("sn"),
		GivenName:     r.FormValue("givenName"),
		SN:            r.FormValue("sn"),
		Mail:          r.FormValue("mail"),
		UIDNumber:     uidNum,
		GIDNumber:     gidNum,
		HomeDirectory: r.FormValue("homeDirectory"),
		LoginShell:    r.FormValue("loginShell"),
	}

	if user.HomeDirectory == "" {
		user.HomeDirectory = "/home/" + user.UID
	}
	if user.LoginShell == "" {
		user.LoginShell = "/bin/bash"
	}

	if err := h.deps.LDAP.CreateUser(user); err != nil {
		content := struct {
			IsEdit bool
			Action string
			User   ldap.User
			Error  string
		}{false, "/users", *user, err.Error()}
		data := pageDataFromRequest(r, "Create User", content)
		h.renderer.renderPage(w, "user_form", data)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) actionUpdateUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	r.ParseForm()
	uidNum, _ := strconv.Atoi(r.FormValue("uidNumber"))
	gidNum, _ := strconv.Atoi(r.FormValue("gidNumber"))

	user := &ldap.User{
		UID:           uid,
		CN:            r.FormValue("givenName") + " " + r.FormValue("sn"),
		GivenName:     r.FormValue("givenName"),
		SN:            r.FormValue("sn"),
		Mail:          r.FormValue("mail"),
		UIDNumber:     uidNum,
		GIDNumber:     gidNum,
		HomeDirectory: r.FormValue("homeDirectory"),
		LoginShell:    r.FormValue("loginShell"),
	}

	if err := h.deps.LDAP.UpdateUser(uid, user); err != nil {
		content := struct {
			IsEdit bool
			Action string
			User   ldap.User
			Error  string
		}{true, "/users/" + uid, *user, err.Error()}
		data := pageDataFromRequest(r, "Edit User", content)
		h.renderer.renderPage(w, "user_form", data)
		return
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) actionDisableUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	h.deps.LDAP.DisableUser(uid)
	// Revoke FIDO2 credentials
	h.deps.Repo.DeleteFIDO2Credentials(uid)
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) actionEnableUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	r.ParseForm()
	confirm := r.FormValue("confirm")
	if confirm != "yesiagree" {
		http.Error(w, "confirmation required", http.StatusBadRequest)
		return
	}
	h.deps.LDAP.EnableUser(uid, "/bin/bash")
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) actionImportUsers(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(10 << 20) // 10MB max
	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "file required", http.StatusBadRequest)
		return
	}
	defer file.Close()

	var users []ldap.User
	if strings.HasSuffix(header.Filename, ".json") {
		data, _ := io.ReadAll(file)
		json.Unmarshal(data, &users)
	} else {
		reader := csv.NewReader(file)
		records, _ := reader.ReadAll()
		for i, rec := range records {
			if i == 0 {
				continue // skip header
			}
			if len(rec) < 6 {
				continue
			}
			uidNum, _ := strconv.Atoi(rec[4])
			gidNum, _ := strconv.Atoi(rec[5])
			shell := "/bin/bash"
			homeDir := "/home/" + rec[0]
			if len(rec) > 6 {
				homeDir = rec[6]
			}
			if len(rec) > 7 {
				shell = rec[7]
			}
			users = append(users, ldap.User{
				UID:           rec[0],
				GivenName:     rec[1],
				SN:            rec[2],
				CN:            rec[1] + " " + rec[2],
				Mail:          rec[3],
				UIDNumber:     uidNum,
				GIDNumber:     gidNum,
				HomeDirectory: homeDir,
				LoginShell:    shell,
			})
		}
	}

	for i := range users {
		h.deps.LDAP.CreateUser(&users[i])
	}
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) actionCreateGroup(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	gidNum, _ := strconv.Atoi(r.FormValue("gidNumber"))
	group := &ldap.Group{
		CN:        r.FormValue("cn"),
		Type:      r.FormValue("type"),
		GIDNumber: gidNum,
	}
	if err := h.deps.LDAP.CreateGroup(group); err != nil {
		content := struct {
			IsEdit  bool
			Action  string
			Group   ldap.Group
			Members []string
			Error   string
		}{false, "/groups", *group, nil, err.Error()}
		data := pageDataFromRequest(r, "Create Group", content)
		h.renderer.renderPage(w, "group_form", data)
		return
	}
	http.Redirect(w, r, "/groups", http.StatusFound)
}

func (h *handlers) actionUpdateGroup(w http.ResponseWriter, r *http.Request) {
	cn := chi.URLParam(r, "cn")
	r.ParseForm()
	members := strings.Split(r.FormValue("members"), "\n")
	var cleaned []string
	for _, m := range members {
		m = strings.TrimSpace(m)
		if m != "" {
			cleaned = append(cleaned, m)
		}
	}
	h.deps.LDAP.UpdateGroupMembers(cn, cleaned)
	http.Redirect(w, r, "/groups", http.StatusFound)
}

func (h *handlers) actionDeleteGroup(w http.ResponseWriter, r *http.Request) {
	cn := chi.URLParam(r, "cn")
	r.ParseForm()
	confirm := r.FormValue("confirm")
	if confirm != "yesiagree" {
		http.Error(w, "confirmation required", http.StatusBadRequest)
		return
	}
	h.deps.LDAP.DeleteGroup(cn)
	http.Redirect(w, r, "/groups", http.StatusFound)
}

func (h *handlers) actionAddMember(w http.ResponseWriter, r *http.Request) {
	cn := chi.URLParam(r, "cn")
	r.ParseForm()
	newMember := r.FormValue("new_member")
	if newMember == "" {
		http.Redirect(w, r, "/groups/"+cn+"/edit", http.StatusFound)
		return
	}
	group, err := h.deps.LDAP.GetGroup(cn)
	if err != nil || group == nil {
		http.Error(w, "group not found", http.StatusNotFound)
		return
	}
	group.Members = append(group.Members, newMember)
	h.deps.LDAP.UpdateGroupMembers(cn, group.Members)
	http.Redirect(w, r, "/groups/"+cn+"/edit", http.StatusFound)
}

func (h *handlers) actionSignSSH(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	pubkey := r.FormValue("pubkey")
	if pubkey == "" {
		http.Error(w, "public key required", http.StatusBadRequest)
		return
	}

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	principal := emailToUID(claims.Email)

	cert, err := h.deps.CA.SignPublicKey([]byte(pubkey), principal, 43200) // 12h default
	if err != nil {
		http.Error(w, "signing failed: "+err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/plain")
	w.Write(cert)
}

func (h *handlers) actionRegisterFIDO2(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	credential := strings.TrimSpace(r.FormValue("credential"))
	if credential == "" {
		http.Error(w, "credential required", http.StatusBadRequest)
		return
	}

	claims := auth.GetClaims(r.Context())
	uid := r.FormValue("uid")
	if uid == "" {
		uid = emailToUID(claims.Email)
	}

	err := h.deps.Repo.CreateFIDO2Credential(&db.FIDO2Credential{
		UID:            uid,
		CredentialData: credential,
	})
	if err != nil {
		http.Error(w, "registration failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/fido2", http.StatusFound)
}

func (h *handlers) actionCreateServiceAccount(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	description := r.FormValue("description")
	role := r.FormValue("role")

	if description == "" || role == "" {
		http.Error(w, "description and role required", http.StatusBadRequest)
		return
	}

	// Generate credentials
	clientID := generateRandomHex(16)
	clientSecret := generateRandomHex(32)

	err := h.deps.Repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID:         clientID,
		ClientSecretHash: hashSecret(clientSecret),
		Description:      description,
		Role:             role,
	})
	if err != nil {
		http.Error(w, "creation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Render page with secret shown
	content := struct {
		NewSecret   string
		NewClientID string
	}{
		NewSecret:   clientSecret,
		NewClientID: clientID,
	}
	data := pageDataFromRequest(r, "Service Accounts", content)
	h.renderer.renderPage(w, "service_accounts", data)
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func hashSecret(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}
