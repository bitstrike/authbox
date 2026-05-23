package ldap

import (
	"fmt"
	"strconv"

	goldap "github.com/go-ldap/ldap/v3"
)

type Group struct {
	CN        string   `json:"cn"`
	Type      string   `json:"type"` // "posixGroup" or "groupOfNames"
	GIDNumber int      `json:"gidNumber,omitempty"`
	Members   []string `json:"members"`
}

func (c *Client) CreateGroup(g *Group) error {
	dn := fmt.Sprintf("cn=%s,ou=groups,%s", g.CN, c.baseDN)
	req := goldap.NewAddRequest(dn, nil)

	if g.Type == "posixGroup" {
		req.Attribute("objectClass", []string{"top", "posixGroup"})
		req.Attribute("cn", []string{g.CN})
		req.Attribute("gidNumber", []string{strconv.Itoa(g.GIDNumber)})
		if len(g.Members) > 0 {
			req.Attribute("memberUid", g.Members)
		}
	} else {
		req.Attribute("objectClass", []string{"top", "groupOfNames"})
		req.Attribute("cn", []string{g.CN})
		if len(g.Members) > 0 {
			req.Attribute("member", g.Members)
		} else {
			// groupOfNames requires at least one member
			req.Attribute("member", []string{dn})
		}
	}
	return c.Add(req)
}

func (c *Client) GetGroup(cn string) (*Group, error) {
	dn := fmt.Sprintf("ou=groups,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		1, 0, false,
		fmt.Sprintf("(cn=%s)", goldap.EscapeFilter(cn)),
		[]string{"cn", "objectClass", "gidNumber", "memberUid", "member"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return nil, nil
	}
	return entryToGroup(result.Entries[0]), nil
}

func (c *Client) ListGroups(offset, limit int) ([]Group, int, error) {
	dn := fmt.Sprintf("ou=groups,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		"(|(objectClass=posixGroup)(objectClass=groupOfNames))",
		[]string{"cn", "objectClass", "gidNumber", "memberUid", "member"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return nil, 0, err
	}

	total := len(result.Entries)
	end := offset + limit
	if end > total {
		end = total
	}
	if offset > total {
		return []Group{}, total, nil
	}

	var groups []Group
	for _, entry := range result.Entries[offset:end] {
		groups = append(groups, *entryToGroup(entry))
	}
	return groups, total, nil
}

func (c *Client) UpdateGroupMembers(cn string, members []string) error {
	dn := fmt.Sprintf("cn=%s,ou=groups,%s", cn, c.baseDN)
	group, err := c.GetGroup(cn)
	if err != nil {
		return err
	}
	if group == nil {
		return fmt.Errorf("group %s not found", cn)
	}

	req := goldap.NewModifyRequest(dn, nil)
	if group.Type == "posixGroup" {
		req.Replace("memberUid", members)
	} else {
		if len(members) == 0 {
			members = []string{dn}
		}
		req.Replace("member", members)
	}
	return c.Modify(req)
}

func (c *Client) DeleteGroup(cn string) error {
	dn := fmt.Sprintf("cn=%s,ou=groups,%s", cn, c.baseDN)
	del := goldap.NewDelRequest(dn, nil)
	return c.conn.Del(del)
}

func (c *Client) GIDExists(gidNumber int) (bool, error) {
	dn := fmt.Sprintf("ou=groups,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		1, 0, false,
		fmt.Sprintf("(gidNumber=%d)", gidNumber),
		[]string{"cn"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return false, err
	}
	return len(result.Entries) > 0, nil
}

// GetUserGroups returns groupOfNames groups that contain the user DN.
func (c *Client) GetUserGroups(uid string) ([]string, error) {
	userDN := fmt.Sprintf("uid=%s,ou=people,%s", uid, c.baseDN)
	dn := fmt.Sprintf("ou=groups,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		fmt.Sprintf("(&(objectClass=groupOfNames)(member=%s))", goldap.EscapeFilter(userDN)),
		[]string{"cn"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return nil, err
	}
	var groups []string
	for _, entry := range result.Entries {
		groups = append(groups, entry.GetAttributeValue("cn"))
	}
	return groups, nil
}

func entryToGroup(e *goldap.Entry) *Group {
	classes := e.GetAttributeValues("objectClass")
	groupType := "groupOfNames"
	for _, c := range classes {
		if c == "posixGroup" {
			groupType = "posixGroup"
			break
		}
	}

	g := &Group{
		CN:   e.GetAttributeValue("cn"),
		Type: groupType,
	}

	if groupType == "posixGroup" {
		gid, _ := strconv.Atoi(e.GetAttributeValue("gidNumber"))
		g.GIDNumber = gid
		g.Members = e.GetAttributeValues("memberUid")
	} else {
		g.Members = e.GetAttributeValues("member")
	}
	return g
}
