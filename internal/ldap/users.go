// users.go implements LDAP operations for user management: create, get, list,
// update, disable (set /sbin/nologin), enable (restore shell), and UID
// existence checks. The Disabled field is derived from loginShell at read time.
package ldap

import (
	"fmt"
	"strconv"

	goldap "github.com/go-ldap/ldap/v3"
)

type User struct {
	UID             string `json:"uid"`
	CN              string `json:"cn"`
	SN              string `json:"sn"`
	GivenName       string `json:"givenName"`
	Mail            string `json:"mail"`
	UIDNumber       int    `json:"uidNumber"`
	GIDNumber       int    `json:"gidNumber"`
	HomeDirectory   string `json:"homeDirectory"`
	LoginShell      string `json:"loginShell"`
	EmployeeType    string `json:"employeeType,omitempty"`
	TelephoneNumber string `json:"telephoneNumber,omitempty"`
	Mobile          string `json:"mobile,omitempty"`
	HomePhone       string `json:"homePhone,omitempty"`
	Fax             string `json:"facsimileTelephoneNumber,omitempty"`
	Pager           string `json:"pager,omitempty"`
	Disabled        bool   `json:"disabled"`
}

func (c *Client) CreateUser(u *User) error {
	dn := fmt.Sprintf("uid=%s,ou=people,%s", u.UID, c.baseDN)
	req := goldap.NewAddRequest(dn, nil)

	if u.UIDNumber > 0 && u.GIDNumber > 0 {
		req.Attribute("objectClass", []string{"top", "inetOrgPerson", "posixAccount"})
	} else {
		req.Attribute("objectClass", []string{"top", "inetOrgPerson"})
	}

	req.Attribute("uid", []string{u.UID})
	req.Attribute("cn", []string{u.CN})
	req.Attribute("sn", []string{u.SN})
	if u.GivenName != "" {
		req.Attribute("givenName", []string{u.GivenName})
	}
	if u.Mail != "" {
		req.Attribute("mail", []string{u.Mail})
	}
	if u.UIDNumber > 0 && u.GIDNumber > 0 {
		req.Attribute("uidNumber", []string{strconv.Itoa(u.UIDNumber)})
		req.Attribute("gidNumber", []string{strconv.Itoa(u.GIDNumber)})
		req.Attribute("homeDirectory", []string{u.HomeDirectory})
		req.Attribute("loginShell", []string{u.LoginShell})
	}
	if u.EmployeeType != "" {
		req.Attribute("employeeType", []string{u.EmployeeType})
	}
	if u.TelephoneNumber != "" {
		req.Attribute("telephoneNumber", []string{u.TelephoneNumber})
	}
	if u.Mobile != "" {
		req.Attribute("mobile", []string{u.Mobile})
	}
	if u.HomePhone != "" {
		req.Attribute("homePhone", []string{u.HomePhone})
	}
	if u.Fax != "" {
		req.Attribute("facsimileTelephoneNumber", []string{u.Fax})
	}
	if u.Pager != "" {
		req.Attribute("pager", []string{u.Pager})
	}
	return c.Add(req)
}

func (c *Client) GetUser(uid string) (*User, error) {
	dn := fmt.Sprintf("ou=people,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		1, 0, false,
		fmt.Sprintf("(uid=%s)", goldap.EscapeFilter(uid)),
		[]string{"uid", "cn", "sn", "givenName", "mail", "uidNumber", "gidNumber", "homeDirectory", "loginShell", "employeeType", "telephoneNumber", "mobile", "homePhone", "facsimileTelephoneNumber", "pager"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return nil, nil
	}
	return entryToUser(result.Entries[0]), nil
}

func (c *Client) ListUsers(offset, limit int) ([]User, int, error) {
	dn := fmt.Sprintf("ou=people,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=inetOrgPerson)",
		[]string{"uid", "cn", "sn", "givenName", "mail", "uidNumber", "gidNumber", "homeDirectory", "loginShell", "employeeType", "telephoneNumber", "mobile", "homePhone", "facsimileTelephoneNumber", "pager"},
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
		return []User{}, total, nil
	}

	var users []User
	for _, entry := range result.Entries[offset:end] {
		users = append(users, *entryToUser(entry))
	}
	return users, total, nil
}

func (c *Client) UpdateUser(uid string, u *User) error {
	dn := fmt.Sprintf("uid=%s,ou=people,%s", uid, c.baseDN)
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace("cn", []string{u.CN})
	req.Replace("sn", []string{u.SN})
	if u.GivenName != "" {
		req.Replace("givenName", []string{u.GivenName})
	}
	if u.Mail != "" {
		req.Replace("mail", []string{u.Mail})
	}
	req.Replace("uidNumber", []string{strconv.Itoa(u.UIDNumber)})
	req.Replace("gidNumber", []string{strconv.Itoa(u.GIDNumber)})
	req.Replace("homeDirectory", []string{u.HomeDirectory})
	req.Replace("loginShell", []string{u.LoginShell})
	if u.EmployeeType != "" {
		req.Replace("employeeType", []string{u.EmployeeType})
	}
	// Phone attributes: replace if set, delete if cleared
	replaceOrDelete := func(attr, val string) {
		if val != "" {
			req.Replace(attr, []string{val})
		} else {
			req.Replace(attr, []string{})
		}
	}
	replaceOrDelete("telephoneNumber", u.TelephoneNumber)
	replaceOrDelete("mobile", u.Mobile)
	replaceOrDelete("homePhone", u.HomePhone)
	replaceOrDelete("facsimileTelephoneNumber", u.Fax)
	replaceOrDelete("pager", u.Pager)
	return c.Modify(req)
}

func (c *Client) DisableUser(uid string) error {
	dn := fmt.Sprintf("uid=%s,ou=people,%s", uid, c.baseDN)
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace("loginShell", []string{"/sbin/nologin"})
	return c.Modify(req)
}

func (c *Client) EnableUser(uid string, shell string) error {
	dn := fmt.Sprintf("uid=%s,ou=people,%s", uid, c.baseDN)
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace("loginShell", []string{shell})
	return c.Modify(req)
}

func (c *Client) DeleteUser(uid string) error {
	dn := fmt.Sprintf("uid=%s,ou=people,%s", uid, c.baseDN)
	del := goldap.NewDelRequest(dn, nil)
	return c.conn.Del(del)
}

func (c *Client) UIDExists(uidNumber int) (bool, error) {
	dn := fmt.Sprintf("ou=people,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		1, 0, false,
		fmt.Sprintf("(uidNumber=%d)", uidNumber),
		[]string{"uid"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return false, err
	}
	return len(result.Entries) > 0, nil
}

// NextAvailableUID finds the next unused uidNumber within the given range.
// Also checks against existing group gidNumbers to avoid collisions when
// UID and GID are set to the same value.
func (c *Client) NextAvailableUID(rangeStart, rangeEnd int) (int, error) {
	dn := fmt.Sprintf("ou=people,%s", c.baseDN)
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=posixAccount)",
		[]string{"uidNumber", "gidNumber"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return 0, err
	}

	used := make(map[int]bool)
	for _, entry := range result.Entries {
		num, _ := strconv.Atoi(entry.GetAttributeValue("uidNumber"))
		used[num] = true
		gid, _ := strconv.Atoi(entry.GetAttributeValue("gidNumber"))
		used[gid] = true
	}

	// Also check group gidNumbers
	groupDN := fmt.Sprintf("ou=groups,%s", c.baseDN)
	greq := goldap.NewSearchRequest(
		groupDN,
		goldap.ScopeSingleLevel,
		goldap.NeverDerefAliases,
		0, 0, false,
		"(objectClass=posixGroup)",
		[]string{"gidNumber"},
		nil,
	)
	gresult, err := c.Search(greq)
	if err == nil {
		for _, entry := range gresult.Entries {
			gid, _ := strconv.Atoi(entry.GetAttributeValue("gidNumber"))
			if gid > 0 {
				used[gid] = true
			}
		}
	}

	for i := rangeStart; i <= rangeEnd; i++ {
		if !used[i] {
			return i, nil
		}
	}
	return 0, fmt.Errorf("no available UID in range %d-%d", rangeStart, rangeEnd)
}

func entryToUser(e *goldap.Entry) *User {
	uidNum, _ := strconv.Atoi(e.GetAttributeValue("uidNumber"))
	gidNum, _ := strconv.Atoi(e.GetAttributeValue("gidNumber"))
	shell := e.GetAttributeValue("loginShell")
	return &User{
		UID:             e.GetAttributeValue("uid"),
		CN:              e.GetAttributeValue("cn"),
		SN:              e.GetAttributeValue("sn"),
		GivenName:       e.GetAttributeValue("givenName"),
		Mail:            e.GetAttributeValue("mail"),
		UIDNumber:       uidNum,
		GIDNumber:       gidNum,
		HomeDirectory:   e.GetAttributeValue("homeDirectory"),
		LoginShell:      shell,
		EmployeeType:    e.GetAttributeValue("employeeType"),
		TelephoneNumber: e.GetAttributeValue("telephoneNumber"),
		Mobile:          e.GetAttributeValue("mobile"),
		HomePhone:       e.GetAttributeValue("homePhone"),
		Fax:             e.GetAttributeValue("facsimileTelephoneNumber"),
		Pager:           e.GetAttributeValue("pager"),
		Disabled:        shell == "/sbin/nologin",
	}
}
