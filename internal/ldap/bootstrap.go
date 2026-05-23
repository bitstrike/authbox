package ldap

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
)

type BootstrapConfig struct {
	BaseDN       string
	AdminEmail   string
	SchemaPath   string
}

func Bootstrap(client *Client, cfg BootstrapConfig) error {
	empty, err := client.IsEmpty()
	if err != nil {
		return fmt.Errorf("checking if directory is empty: %w", err)
	}
	if !empty {
		return nil
	}

	if err := createBaseDN(client, cfg.BaseDN); err != nil {
		return fmt.Errorf("creating base DN: %w", err)
	}

	if err := createOUs(client, cfg.BaseDN); err != nil {
		return fmt.Errorf("creating OUs: %w", err)
	}

	if err := createRoleGroups(client, cfg.BaseDN, cfg.AdminEmail); err != nil {
		return fmt.Errorf("creating role groups: %w", err)
	}

	if cfg.AdminEmail != "" {
		if err := createInitialAdmin(client, cfg.BaseDN, cfg.AdminEmail); err != nil {
			return fmt.Errorf("creating initial admin: %w", err)
		}
	}

	if cfg.SchemaPath != "" {
		if err := applyLDIF(client, cfg.SchemaPath, cfg.BaseDN); err != nil {
			return fmt.Errorf("applying schema LDIF: %w", err)
		}
	}

	return nil
}

func createBaseDN(client *Client, baseDN string) error {
	parts := strings.Split(baseDN, ",")
	dc := strings.TrimPrefix(parts[0], "dc=")

	req := goldap.NewAddRequest(baseDN, nil)
	req.Attribute("objectClass", []string{"top", "dcObject", "organization"})
	req.Attribute("dc", []string{dc})
	req.Attribute("o", []string{dc})
	return client.Add(req)
}

func createOUs(client *Client, baseDN string) error {
	ous := []string{"people", "groups", "serviceaccounts"}
	for _, ou := range ous {
		dn := fmt.Sprintf("ou=%s,%s", ou, baseDN)
		req := goldap.NewAddRequest(dn, nil)
		req.Attribute("objectClass", []string{"top", "organizationalUnit"})
		req.Attribute("ou", []string{ou})
		if err := client.Add(req); err != nil {
			return fmt.Errorf("creating ou=%s: %w", ou, err)
		}
	}
	return nil
}

func createRoleGroups(client *Client, baseDN, adminEmail string) error {
	groups := []string{"authbox-admins", "authbox-operators", "authbox-viewers"}
	for _, cn := range groups {
		dn := fmt.Sprintf("cn=%s,ou=groups,%s", cn, baseDN)
		req := goldap.NewAddRequest(dn, nil)
		req.Attribute("objectClass", []string{"top", "groupOfNames"})
		req.Attribute("cn", []string{cn})
		if cn == "authbox-admins" && adminEmail != "" {
			adminDN := fmt.Sprintf("uid=%s,ou=people,%s", emailToUID(adminEmail), baseDN)
			req.Attribute("member", []string{adminDN})
		} else {
			req.Attribute("member", []string{fmt.Sprintf("cn=%s,ou=groups,%s", cn, baseDN)})
		}
		if err := client.Add(req); err != nil {
			return fmt.Errorf("creating cn=%s: %w", cn, err)
		}
	}
	return nil
}

func createInitialAdmin(client *Client, baseDN, email string) error {
	uid := emailToUID(email)
	dn := fmt.Sprintf("uid=%s,ou=people,%s", uid, baseDN)

	req := goldap.NewAddRequest(dn, nil)
	req.Attribute("objectClass", []string{"top", "inetOrgPerson", "posixAccount"})
	req.Attribute("uid", []string{uid})
	req.Attribute("cn", []string{uid})
	req.Attribute("sn", []string{uid})
	req.Attribute("mail", []string{email})
	req.Attribute("uidNumber", []string{"10000"})
	req.Attribute("gidNumber", []string{"10000"})
	req.Attribute("homeDirectory", []string{fmt.Sprintf("/home/%s", uid)})
	req.Attribute("loginShell", []string{"/bin/bash"})
	return client.Add(req)
}

func emailToUID(email string) string {
	parts := strings.Split(email, "@")
	return parts[0]
}

func applyLDIF(client *Client, path, baseDN string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	var currentDN string
	var attrs map[string][]string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "#") || line == "" {
			if currentDN != "" && len(attrs) > 0 {
				if err := addEntry(client, currentDN, attrs); err != nil {
					return fmt.Errorf("adding %s: %w", currentDN, err)
				}
			}
			currentDN = ""
			attrs = nil
			continue
		}

		if strings.HasPrefix(line, "dn: ") {
			currentDN = strings.TrimPrefix(line, "dn: ")
			currentDN = strings.ReplaceAll(currentDN, "dc=example,dc=com", baseDN)
			attrs = make(map[string][]string)
			continue
		}

		if currentDN != "" && strings.Contains(line, ": ") {
			parts := strings.SplitN(line, ": ", 2)
			key := parts[0]
			val := strings.ReplaceAll(parts[1], "dc=example,dc=com", baseDN)
			attrs[key] = append(attrs[key], val)
		}
	}

	if currentDN != "" && len(attrs) > 0 {
		if err := addEntry(client, currentDN, attrs); err != nil {
			return fmt.Errorf("adding %s: %w", currentDN, err)
		}
	}

	return scanner.Err()
}

func addEntry(client *Client, dn string, attrs map[string][]string) error {
	req := goldap.NewAddRequest(dn, nil)
	for k, v := range attrs {
		if k == "dn" {
			continue
		}
		req.Attribute(k, v)
	}
	err := client.Add(req)
	if err != nil && goldap.IsErrorWithCode(err, goldap.LDAPResultEntryAlreadyExists) {
		return nil
	}
	return err
}
