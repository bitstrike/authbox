package ldap

import (
	"fmt"
	"strings"

	"github.com/authbox/authbox/internal/auth"
	goldap "github.com/go-ldap/ldap/v3"
)

// RoleLookup implements auth.RoleLookup using LDAP group membership.
type RoleLookup struct {
	client *Client
}

func NewRoleLookup(client *Client) *RoleLookup {
	return &RoleLookup{client: client}
}

func (r *RoleLookup) GetRolesForUser(email string) ([]auth.Role, error) {
	uid := emailToUID(email)
	userDN := fmt.Sprintf("uid=%s,ou=people,%s", uid, r.client.baseDN)

	// Check if user exists
	user, err := r.client.GetUser(uid)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("user %s not found in directory", email)
	}

	// Search for role groups containing this user
	dn := fmt.Sprintf("ou=groups,%s", r.client.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(&(objectClass=groupOfNames)(cn=authbox-*)(member=%s))", goldap.EscapeFilter(userDN)),
		[]string{"cn"},
		nil,
	)
	result, err := r.client.Search(req)
	if err != nil {
		return nil, err
	}

	var roles []auth.Role
	for _, entry := range result.Entries {
		cn := entry.GetAttributeValue("cn")
		switch {
		case strings.HasSuffix(cn, "-admins"):
			roles = append(roles, auth.RoleAdmin)
		case strings.HasSuffix(cn, "-operators"):
			roles = append(roles, auth.RoleOperator)
		case strings.HasSuffix(cn, "-viewers"):
			roles = append(roles, auth.RoleViewer)
		}
	}
	return roles, nil
}
