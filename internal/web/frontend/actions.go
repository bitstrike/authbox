// actions.go contains POST handlers for form submissions and mutating
// operations: creating/updating/disabling users, managing groups, signing SSH
// keys, registering FIDO2 credentials, bulk import, and service account
// creation. Each action validates input, calls the appropriate backend
// (LDAP/SQLite/CA), and redirects or re-renders the form on error.
package frontend

import (
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/authbox/authbox/internal/auth"
	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/flash"
	"github.com/authbox/authbox/internal/ldap"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"
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
		r.Post("/users/{uid}/delete", f.h.actionDeleteUser)
		r.Post("/users/bulk/disable", f.h.actionBulkDisableUsers)
		r.Post("/users/bulk/delete", f.h.actionBulkDeleteUsers)
		r.Post("/users/bulk/change-type", f.h.actionBulkChangeType)
		r.Post("/users/bulk/add-to-group", f.h.actionBulkAddToGroup)
		r.Post("/users/bulk/remove-from-group", f.h.actionBulkRemoveFromGroup)
		r.Post("/users/bulk/export", f.h.actionBulkExportUsers)
	})

	// Groups
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/groups", f.h.actionCreateGroup)
		r.Post("/groups/{cn}", f.h.actionUpdateGroup)
		r.Post("/groups/{cn}/delete", f.h.actionDeleteGroup)
		r.Post("/groups/{cn}/members", f.h.actionAddMember)
		r.Post("/groups/bulk/delete", f.h.actionBulkDeleteGroups)
	})

	// SSH
	r.Post("/ssh/sign", f.h.actionSignSSH)
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/ssh/bulk/delete", f.h.actionBulkDeleteCerts)
	})

	// FIDO2
	r.Post("/fido2/register", f.h.actionRegisterFIDO2)
	r.Post("/fido2/credentials/{id}/revoke", f.h.actionRevokeFIDO2)
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/fido2/bulk/revoke", f.h.actionBulkRevokeFIDO2)
	})

	// Service accounts (admin)
	r.Group(func(r chi.Router) {
		r.Use(requireFrontendRole(auth.RoleAdmin))
		r.Post("/service-accounts", f.h.actionCreateServiceAccount)
		r.Post("/service-accounts/{clientID}/delete", f.h.actionDeleteServiceAccount)
		r.Post("/service-accounts/bulk/delete", f.h.actionBulkDeleteServiceAccounts)
	})
}

func (h *handlers) actionCreateUser(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	uidNum, _ := strconv.Atoi(r.FormValue("uidNumber"))
	gidNum, _ := strconv.Atoi(r.FormValue("gidNumber"))

	user := &ldap.User{
		UID:             r.FormValue("uid"),
		CN:              r.FormValue("givenName") + " " + r.FormValue("sn"),
		GivenName:       r.FormValue("givenName"),
		SN:              r.FormValue("sn"),
		Mail:            r.FormValue("mail"),
		UIDNumber:       uidNum,
		GIDNumber:       gidNum,
		HomeDirectory:   r.FormValue("homeDirectory"),
		LoginShell:      r.FormValue("loginShell"),
		EmployeeType:    r.FormValue("employeeType"),
		TelephoneNumber: r.FormValue("telephoneNumber"),
		Mobile:          r.FormValue("mobile"),
		HomePhone:       r.FormValue("homePhone"),
		Fax:             r.FormValue("fax"),
		Pager:           r.FormValue("pager"),
	}

	if user.HomeDirectory == "" {
		user.HomeDirectory = "/home/" + user.UID
	}
	if user.LoginShell == "" {
		user.LoginShell = "/bin/bash"
	}

	// Validate UID/GID uniqueness
	if user.UIDNumber > 0 {
		exists, err := h.deps.LDAP.UIDExists(user.UIDNumber)
		if err == nil && exists {
			h.renderCreateUserError(w, r, user, "uidNumber already in use")
			return
		}
	}
	if user.GIDNumber > 0 {
		gidUsedByGroup, err := h.deps.LDAP.GIDExists(user.GIDNumber)
		if err == nil && gidUsedByGroup {
			h.renderCreateUserError(w, r, user, "gidNumber already in use by a group")
			return
		}
	}

	if err := h.deps.LDAP.CreateUser(user); err != nil {
		h.renderCreateUserError(w, r, user, err.Error())
		return
	}
	flash.Set(w, flash.Success, "User "+user.UID+" created")
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) renderCreateUserError(w http.ResponseWriter, r *http.Request, user *ldap.User, errMsg string) {
	employeeTypes, _ := h.deps.Repo.ListEmployeeTypes()
	rangeStart, _ := strconv.Atoi(h.deps.Config.UIDRangeStart)
	rangeEnd, _ := strconv.Atoi(h.deps.Config.UIDRangeEnd)
	content := struct {
		IsEdit        bool
		Action        string
		User          ldap.User
		Error         string
		EmployeeTypes []db.EmployeeType
		UIDRangeStart int
		UIDRangeEnd   int
	}{false, "/users", *user, errMsg, employeeTypes, rangeStart, rangeEnd}
	data := pageDataFromRequest(w, r, "Create User", content)
	h.renderer.renderPage(w, "user_form", data)
}

func (h *handlers) actionUpdateUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	r.ParseForm()
	uidNum, _ := strconv.Atoi(r.FormValue("uidNumber"))
	gidNum, _ := strconv.Atoi(r.FormValue("gidNumber"))

	user := &ldap.User{
		UID:             uid,
		CN:              r.FormValue("givenName") + " " + r.FormValue("sn"),
		GivenName:       r.FormValue("givenName"),
		SN:              r.FormValue("sn"),
		Mail:            r.FormValue("mail"),
		UIDNumber:       uidNum,
		GIDNumber:       gidNum,
		HomeDirectory:   r.FormValue("homeDirectory"),
		LoginShell:      r.FormValue("loginShell"),
		EmployeeType:    r.FormValue("employeeType"),
		TelephoneNumber: r.FormValue("telephoneNumber"),
		Mobile:          r.FormValue("mobile"),
		HomePhone:       r.FormValue("homePhone"),
		Fax:             r.FormValue("fax"),
		Pager:           r.FormValue("pager"),
	}

	// Validate UID/GID uniqueness if changed
	existing, _ := h.deps.LDAP.GetUser(uid)
	if user.UIDNumber > 0 && (existing == nil || user.UIDNumber != existing.UIDNumber) {
		exists, err := h.deps.LDAP.UIDExists(user.UIDNumber)
		if err == nil && exists {
			h.renderEditUserError(w, r, uid, user, "uidNumber already in use")
			return
		}
	}
	if user.GIDNumber > 0 && (existing == nil || user.GIDNumber != existing.GIDNumber) {
		gidUsedByGroup, err := h.deps.LDAP.GIDExists(user.GIDNumber)
		if err == nil && gidUsedByGroup {
			h.renderEditUserError(w, r, uid, user, "gidNumber already in use by a group")
			return
		}
	}

	if err := h.deps.LDAP.UpdateUser(uid, user); err != nil {
		h.renderEditUserError(w, r, uid, user, err.Error())
		return
	}
	flash.Set(w, flash.Success, "User "+uid+" updated")
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) renderEditUserError(w http.ResponseWriter, r *http.Request, uid string, user *ldap.User, errMsg string) {
	employeeTypes, _ := h.deps.Repo.ListEmployeeTypes()
	rangeStart, _ := strconv.Atoi(h.deps.Config.UIDRangeStart)
	rangeEnd, _ := strconv.Atoi(h.deps.Config.UIDRangeEnd)
	content := struct {
		IsEdit        bool
		Action        string
		User          ldap.User
		Error         string
		EmployeeTypes []db.EmployeeType
		UIDRangeStart int
		UIDRangeEnd   int
	}{true, "/users/" + uid, *user, errMsg, employeeTypes, rangeStart, rangeEnd}
	data := pageDataFromRequest(w, r, "Edit User", content)
	h.renderer.renderPage(w, "user_form", data)
}

func (h *handlers) actionDeleteUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	r.ParseForm()
	confirm := r.FormValue("confirm")
	if confirm != "yesiagree" {
		http.Error(w, "confirmation required", http.StatusBadRequest)
		return
	}

	// Only allow deletion of disabled accounts (contacts are always deletable)
	user, err := h.deps.LDAP.GetUser(uid)
	if err != nil || user == nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if !user.Disabled && user.EmployeeType != "contact" {
		http.Error(w, "user must be disabled before deletion", http.StatusBadRequest)
		return
	}

	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}
	h.deps.Log.Info("user deleted", "uid", uid, "by", actor)

	if err := h.deps.LDAP.DeleteUser(uid); err != nil {
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	flash.Set(w, flash.Success, "User "+uid+" deleted")
	http.Redirect(w, r, "/users", http.StatusFound)
}

func (h *handlers) actionDisableUser(w http.ResponseWriter, r *http.Request) {
	uid := chi.URLParam(r, "uid")
	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}

	user, _ := h.deps.LDAP.GetUser(uid)
	if user != nil && user.EmployeeType == "contact" {
		flash.Set(w, flash.Error, "Contacts cannot be disabled (no login capability)")
		http.Redirect(w, r, "/users/"+uid+"/edit", http.StatusFound)
		return
	}

	h.deps.Log.Info("user disabled", "uid", uid, "by", actor)
	h.deps.LDAP.DisableUser(uid)
	// Revoke FIDO2 credentials
	h.deps.Repo.DeleteFIDO2Credentials(uid)
	flash.Set(w, flash.Success, "User "+uid+" disabled")
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
	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}
	h.deps.Log.Info("user enabled", "uid", uid, "by", actor)
	h.deps.LDAP.EnableUser(uid, "/bin/bash")
	flash.Set(w, flash.Success, "User "+uid+" enabled")
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

	rangeStart, _ := strconv.Atoi(h.deps.Config.UIDRangeStart)
	rangeEnd, _ := strconv.Atoi(h.deps.Config.UIDRangeEnd)

	var users []ldap.User
	var errors []string

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
			employeeType := ""
			if len(rec) > 8 {
				employeeType = rec[8]
			}
			telephoneNumber := ""
			if len(rec) > 9 {
				telephoneNumber = rec[9]
			}
			mobile := ""
			if len(rec) > 10 {
				mobile = rec[10]
			}
			homePhone := ""
			if len(rec) > 11 {
				homePhone = rec[11]
			}
			fax := ""
			if len(rec) > 12 {
				fax = rec[12]
			}
			pager := ""
			if len(rec) > 13 {
				pager = rec[13]
			}

			// Contact-type handling
			if employeeType == "contact" {
				uidNum = 0
				gidNum = 0
				homeDir = ""
				shell = "/sbin/nologin"
			}

			// UID/GID range validation for non-contact types
			if employeeType != "contact" {
				if uidNum < rangeStart || uidNum > rangeEnd {
					errors = append(errors, fmt.Sprintf("Row %d: UID %d outside range %d-%d", i+1, uidNum, rangeStart, rangeEnd))
				}
				if gidNum < rangeStart || gidNum > rangeEnd {
					errors = append(errors, fmt.Sprintf("Row %d: GID %d outside range %d-%d", i+1, gidNum, rangeStart, rangeEnd))
				}
			}

			users = append(users, ldap.User{
				UID:             rec[0],
				GivenName:       rec[1],
				SN:              rec[2],
				CN:              rec[1] + " " + rec[2],
				Mail:            rec[3],
				UIDNumber:       uidNum,
				GIDNumber:       gidNum,
				HomeDirectory:   homeDir,
				LoginShell:      shell,
				EmployeeType:    employeeType,
				TelephoneNumber: telephoneNumber,
				Mobile:          mobile,
				HomePhone:       homePhone,
				Fax:             fax,
				Pager:           pager,
			})
		}
	}

	// Abort if validation errors
	if len(errors) > 0 {
		flash.Set(w, flash.Error, "Import aborted: "+strings.Join(errors, "; "))
		http.Redirect(w, r, "/users/import", http.StatusFound)
		return
	}

	for i := range users {
		h.deps.LDAP.CreateUser(&users[i])
	}
	flash.Set(w, flash.Success, fmt.Sprintf("%d users imported", len(users)))
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
		data := pageDataFromRequest(w, r, "Create Group", content)
		h.renderer.renderPage(w, "group_form", data)
		return
	}
	flash.Set(w, flash.Success, "Group "+group.CN+" created")
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
	flash.Set(w, flash.Success, "Group "+cn+" updated")
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
	flash.Set(w, flash.Success, "Group "+cn+" deleted")
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
	flash.Set(w, flash.Success, "Member "+newMember+" added to "+cn)
	http.Redirect(w, r, "/groups/"+cn+"/edit", http.StatusFound)
}

func (h *handlers) actionSignSSH(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	pubkey := r.FormValue("pubkey")
	if pubkey == "" {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="p-3 rounded bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-sm">Public key required</div>`))
		return
	}

	claims := auth.GetClaims(r.Context())
	if claims == nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<div class="p-3 rounded bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-sm">Unauthorized</div>`))
		return
	}
	principal := emailToUID(claims.Email)

	// Rate limit check
	allowed, remaining, resetIn := h.signLimiter.allow(principal)
	if !allowed {
		w.Header().Set("Content-Type", "text/html")
		w.Header().Set("HX-Retarget", "#ssh-error")
		w.Write([]byte(fmt.Sprintf(
			`<div class="p-3 rounded bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-sm">Rate limit exceeded (max %d per hour). Try again in %d minutes.</div>`,
			maxCertsPerHour, int(resetIn.Minutes())+1,
		)))
		return
	}

	cert, err := h.deps.CA.SignPublicKey([]byte(pubkey), principal, 43200) // 12h default
	if err != nil {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(fmt.Sprintf(`<div class="p-3 rounded bg-red-100 dark:bg-red-900 text-red-800 dark:text-red-200 text-sm">Signing failed: %s</div>`, escHTML(err.Error()))))
		return
	}

	certStr := strings.TrimSpace(string(cert))

	// Record in audit log
	serial := generateRandomHex(8)
	h.deps.Repo.CreateSSHCert(&db.SSHCert{
		Username:  principal,
		Serial:    serial,
		Principal: principal,
		ExpiresAt: time.Now().Add(12 * time.Hour),
	})
	h.signLimiter.record(principal)
	remaining = h.signLimiter.remaining(principal)

	w.Header().Set("Content-Type", "text/html")
	w.Header().Set("HX-Trigger", "cert-issued")
	var sb strings.Builder
	sb.WriteString(`<div class="p-4 border rounded dark:border-gray-700 bg-gray-50 dark:bg-gray-800 space-y-3">`)
	sb.WriteString(`<h3 class="font-semibold text-sm">Certificate Issued</h3>`)
	sb.WriteString(`<div class="grid grid-cols-2 gap-2 text-sm">`)
	sb.WriteString(fmt.Sprintf(`<div><span class="text-gray-500">Principal:</span> <strong>%s</strong></div>`, escHTML(principal)))
	sb.WriteString(`<div><span class="text-gray-500">TTL:</span> <strong>12 hours</strong></div>`)
	sb.WriteString(fmt.Sprintf(`<div><span class="text-gray-500">Type:</span> <strong>%s</strong></div>`, escHTML(strings.SplitN(certStr, " ", 2)[0])))
	sb.WriteString(`</div>`)
	sb.WriteString(`<div><label class="label">Signed Certificate</label>`)
	sb.WriteString(fmt.Sprintf(`<textarea id="ssh-cert-output" rows="4" class="input font-mono text-xs" readonly>%s</textarea></div>`, escHTML(certStr)))
	sb.WriteString(`<div class="flex space-x-2">`)
	sb.WriteString(`<button onclick="navigator.clipboard.writeText(document.getElementById('ssh-cert-output').value)" class="btn btn-primary text-sm">Copy to Clipboard</button>`)
	sb.WriteString(`<a href="data:text/plain,` + certStr + `" download="id_ed25519-cert.pub" class="btn btn-secondary text-sm">Download</a>`)
	sb.WriteString(`</div>`)
	sb.WriteString(fmt.Sprintf(`<p class="text-xs text-gray-500 dark:text-gray-400">%d of %d signs remaining this hour</p>`, remaining, maxCertsPerHour))
	sb.WriteString(`</div>`)
	w.Write([]byte(sb.String()))
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
	flash.Set(w, flash.Success, "FIDO2 credential registered for "+uid)
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

	hash, err := bcrypt.GenerateFromPassword([]byte(clientSecret), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "failed to hash secret", http.StatusInternalServerError)
		return
	}

	err = h.deps.Repo.CreateServiceAccount(&db.ServiceAccount{
		ClientID:         clientID,
		ClientSecretHash: string(hash),
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
	data := pageDataFromRequest(w, r, "Service Accounts", content)
	h.renderer.renderPage(w, "service_accounts", data)
}

func generateRandomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (h *handlers) actionDeleteServiceAccount(w http.ResponseWriter, r *http.Request) {
	clientID := chi.URLParam(r, "clientID")
	h.deps.Repo.DeleteServiceAccount(clientID)
	w.Header().Set("HX-Trigger", `{"showFlash":{"type":"success","text":"Service account deleted"}}`)
	h.partialServiceAccountList(w, r)
}

func (h *handlers) actionRevokeFIDO2(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	h.deps.Repo.DeleteFIDO2CredentialByID(id)
	w.Header().Set("HX-Trigger", `{"showFlash":{"type":"success","text":"FIDO2 credential revoked"}}`)
	h.partialFIDO2List(w, r)
}

// Bulk action request body
type bulkRequest struct {
	IDs   []string `json:"ids"`
	Value string   `json:"value,omitempty"`
}

func (h *handlers) actionBulkDisableUsers(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No users selected"})
		return
	}

	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}

	disabled := 0
	for _, uid := range req.IDs {
		user, err := h.deps.LDAP.GetUser(uid)
		if err != nil || user == nil || user.Disabled || user.EmployeeType == "contact" {
			continue
		}
		h.deps.LDAP.DisableUser(uid)
		h.deps.Repo.DeleteFIDO2Credentials(uid)
		h.deps.Log.Info("user disabled (bulk)", "uid", uid, "by", actor)
		disabled++
	}

	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d users disabled", disabled),
	})
}

func (h *handlers) actionBulkDeleteUsers(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No users selected"})
		return
	}

	claims := auth.GetClaims(r.Context())
	actor := ""
	if claims != nil {
		actor = claims.Email
	}

	deleted := 0
	skipped := 0
	for _, uid := range req.IDs {
		user, err := h.deps.LDAP.GetUser(uid)
		if err != nil || user == nil {
			continue
		}
		if !user.Disabled && user.EmployeeType != "contact" {
			skipped++
			continue
		}
		if err := h.deps.LDAP.DeleteUser(uid); err == nil {
			h.deps.Log.Info("user deleted (bulk)", "uid", uid, "by", actor)
			deleted++
		}
	}

	msg := fmt.Sprintf("%d users deleted", deleted)
	if skipped > 0 {
		msg += fmt.Sprintf(" (%d skipped - not disabled)", skipped)
	}
	respondJSON(w, http.StatusOK, map[string]string{"type": "success", "message": msg})
}

func respondJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *handlers) actionBulkDeleteGroups(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No groups selected"})
		return
	}
	deleted := 0
	for _, cn := range req.IDs {
		if err := h.deps.LDAP.DeleteGroup(cn); err == nil {
			deleted++
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d groups deleted", deleted),
	})
}

func (h *handlers) actionBulkDeleteCerts(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No certs selected"})
		return
	}
	deleted := 0
	for _, idStr := range req.IDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		if err := h.deps.Repo.DeleteSSHCert(id); err == nil {
			deleted++
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d certificates deleted", deleted),
	})
}

func (h *handlers) actionBulkRevokeFIDO2(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No credentials selected"})
		return
	}
	revoked := 0
	for _, idStr := range req.IDs {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		if err := h.deps.Repo.DeleteFIDO2CredentialByID(id); err == nil {
			revoked++
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d credentials revoked", revoked),
	})
}

func (h *handlers) actionBulkDeleteServiceAccounts(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No accounts selected"})
		return
	}
	deleted := 0
	for _, clientID := range req.IDs {
		if err := h.deps.Repo.DeleteServiceAccount(clientID); err == nil {
			deleted++
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d service accounts deleted", deleted),
	})
}

func (h *handlers) actionBulkChangeType(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No users selected"})
		return
	}
	if req.Value == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "Employee type required"})
		return
	}
	updated := 0
	for _, uid := range req.IDs {
		user, err := h.deps.LDAP.GetUser(uid)
		if err != nil || user == nil {
			continue
		}
		user.EmployeeType = req.Value
		if err := h.deps.LDAP.UpdateUser(uid, user); err == nil {
			updated++
		}
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d users changed to %s", updated, req.Value),
	})
}

func (h *handlers) actionBulkAddToGroup(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No users selected"})
		return
	}
	if req.Value == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "Group name required"})
		return
	}
	group, err := h.deps.LDAP.GetGroup(req.Value)
	if err != nil || group == nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "Group not found: " + req.Value})
		return
	}
	memberSet := make(map[string]bool)
	for _, m := range group.Members {
		memberSet[m] = true
	}
	added := 0
	for _, uid := range req.IDs {
		if !memberSet[uid] {
			group.Members = append(group.Members, uid)
			memberSet[uid] = true
			added++
		}
	}
	if added > 0 {
		h.deps.LDAP.UpdateGroupMembers(req.Value, group.Members)
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d users added to %s", added, req.Value),
	})
}

func (h *handlers) actionBulkRemoveFromGroup(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "No users selected"})
		return
	}
	if req.Value == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "Group name required"})
		return
	}
	group, err := h.deps.LDAP.GetGroup(req.Value)
	if err != nil || group == nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"type": "error", "message": "Group not found: " + req.Value})
		return
	}
	removeSet := make(map[string]bool)
	for _, uid := range req.IDs {
		removeSet[uid] = true
	}
	var remaining []string
	removed := 0
	for _, m := range group.Members {
		if removeSet[m] {
			removed++
		} else {
			remaining = append(remaining, m)
		}
	}
	if removed > 0 {
		h.deps.LDAP.UpdateGroupMembers(req.Value, remaining)
	}
	respondJSON(w, http.StatusOK, map[string]string{
		"type":    "success",
		"message": fmt.Sprintf("%d users removed from %s", removed, req.Value),
	})
}

func (h *handlers) actionBulkExportUsers(w http.ResponseWriter, r *http.Request) {
	var req bulkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.IDs) == 0 {
		w.Header().Set("Content-Type", "text/csv")
		w.Write([]byte("uid,givenName,sn,mail,uidNumber,gidNumber,homeDirectory,loginShell,employeeType,telephoneNumber,mobile,homePhone,fax,pager\n"))
		return
	}
	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=users-export.csv")
	w.Write([]byte("uid,givenName,sn,mail,uidNumber,gidNumber,homeDirectory,loginShell,employeeType,telephoneNumber,mobile,homePhone,fax,pager\n"))
	for _, uid := range req.IDs {
		user, err := h.deps.LDAP.GetUser(uid)
		if err != nil || user == nil {
			continue
		}
		fmt.Fprintf(w, "%s,%s,%s,%s,%d,%d,%s,%s,%s,%s,%s,%s,%s,%s\n",
			csvEscape(user.UID), csvEscape(user.GivenName), csvEscape(user.SN),
			csvEscape(user.Mail), user.UIDNumber, user.GIDNumber,
			csvEscape(user.HomeDirectory), csvEscape(user.LoginShell),
			csvEscape(user.EmployeeType), csvEscape(user.TelephoneNumber),
			csvEscape(user.Mobile), csvEscape(user.HomePhone),
			csvEscape(user.Fax), csvEscape(user.Pager),
		)
	}
}

func csvEscape(s string) string {
	if strings.ContainsAny(s, ",\"\n") {
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	return s
}
