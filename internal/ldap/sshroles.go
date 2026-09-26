// sshroles.go implements the SSH login role lookup. SSH login roles are
// groupOfNames entries named sshrole-<name> under ou=groups. Membership grants
// the caller an extra SSH certificate principal equal to <name>, which enrolled
// hosts map to a same-named local account (see authbox-cert-check.sh).
//
// This is deliberately SEPARATE from GetRolesForUser (roles.go): app/API roles
// (authbox-admins/operators/viewers) and SSH login roles (sshrole-*) must never
// couple, so an app-management role never implicitly grants host login access.
package ldap

import (
	"fmt"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
)

// sshRolePrefix is the cn prefix that marks a groupOfNames as an SSH login role.
// The part after the prefix becomes an SSH certificate principal.
const sshRolePrefix = "sshrole-"

// GetSSHRolesForUser returns the SSH login role principal names for a user.
// It searches ou=groups for sshrole-* groupOfNames entries that list the user's
// DN as a member and strips the sshrole- prefix to yield the principal name.
// A user with no sshrole-* memberships returns an empty slice (no error).
func (c *Client) GetSSHRolesForUser(uid string) ([]string, error) {
	userDN := c.UserDN(uid)

	req := goldap.NewSearchRequest(
		fmt.Sprintf("ou=groups,%s", c.baseDN),
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(&(objectClass=groupOfNames)(cn=%s*)(member=%s))",
			sshRolePrefix, goldap.EscapeFilter(userDN)),
		[]string{"cn"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return nil, err
	}

	var roles []string
	for _, entry := range result.Entries {
		cn := entry.GetAttributeValue("cn")
		if name := strings.TrimPrefix(cn, sshRolePrefix); name != "" && name != cn {
			roles = append(roles, name)
		}
	}
	return roles, nil
}
