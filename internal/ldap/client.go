package ldap

import (
	"fmt"

	goldap "github.com/go-ldap/ldap/v3"
)

type Client struct {
	conn     *goldap.Conn
	baseDN   string
	adminDN  string
	adminPW  string
}

func NewClient(baseDN, adminPassword string) (*Client, error) {
	c := &Client{
		baseDN:  baseDN,
		adminDN: fmt.Sprintf("cn=admin,%s", baseDN),
		adminPW: adminPassword,
	}
	if err := c.connect(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) connect() error {
	conn, err := goldap.DialURL("ldap://127.0.0.1:3389")
	if err != nil {
		return fmt.Errorf("ldap dial: %w", err)
	}
	c.conn = conn

	if err := c.conn.Bind(c.adminDN, c.adminPW); err != nil {
		c.conn.Close()
		return fmt.Errorf("ldap bind: %w", err)
	}
	return nil
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) Add(req *goldap.AddRequest) error {
	return c.conn.Add(req)
}

func (c *Client) Modify(req *goldap.ModifyRequest) error {
	return c.conn.Modify(req)
}

func (c *Client) Search(req *goldap.SearchRequest) (*goldap.SearchResult, error) {
	return c.conn.Search(req)
}

func (c *Client) BaseDN() string {
	return c.baseDN
}

// Ping checks if the LDAP connection is alive.
func (c *Client) Ping() error {
	req := goldap.NewSearchRequest(
		"",
		goldap.ScopeBaseObject,
		goldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{"1.1"},
		nil,
	)
	_, err := c.conn.Search(req)
	return err
}

func (c *Client) IsEmpty() (bool, error) {
	req := goldap.NewSearchRequest(
		c.baseDN,
		goldap.ScopeBaseObject,
		goldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{"dn"},
		nil,
	)
	result, err := c.conn.Search(req)
	if err != nil {
		if goldap.IsErrorWithCode(err, goldap.LDAPResultNoSuchObject) {
			return true, nil
		}
		return false, err
	}
	return len(result.Entries) == 0, nil
}
