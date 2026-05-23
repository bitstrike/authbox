package ldap

import (
	"fmt"
	"strconv"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
)

// LDAPConfig represents the readable cn=config state.
type LDAPConfig struct {
	BaseDN      string   `json:"base_dn"`
	RootDN      string   `json:"root_dn"`
	ACLs        []string `json:"acls"`
	Modules     []string `json:"modules"`
	TLSEnabled  bool     `json:"tls_enabled"`
	SyncRepl    *SyncReplConfig `json:"sync_repl,omitempty"`
}

// SyncReplConfig holds OpenLDAP syncrepl consumer settings.
type SyncReplConfig struct {
	RID         int    `json:"rid"`
	Provider    string `json:"provider"`
	SearchBase  string `json:"search_base"`
	BindDN      string `json:"bind_dn"`
	Credentials string `json:"credentials,omitempty"`
	Type        string `json:"type"` // refreshOnly or refreshAndPersist
	Interval    string `json:"interval,omitempty"`
}

// GetConfig reads the current cn=config state.
func (c *Client) GetConfig() (*LDAPConfig, error) {
	cfg := &LDAPConfig{
		BaseDN: c.baseDN,
		RootDN: c.adminDN,
	}

	// Read database config
	dbCfg, err := c.getConfigEntry("olcDatabase={1}mdb,cn=config")
	if err == nil {
		cfg.ACLs = dbCfg.GetAttributeValues("olcAccess")
		syncRepl := dbCfg.GetAttributeValues("olcSyncRepl")
		if len(syncRepl) > 0 {
			cfg.SyncRepl = parseSyncRepl(syncRepl[0])
		}
	}

	// Read loaded modules
	modCfg, err := c.getConfigEntry("cn=module{0},cn=config")
	if err == nil {
		cfg.Modules = modCfg.GetAttributeValues("olcModuleLoad")
	}

	// Check TLS
	globalCfg, err := c.getConfigEntry("cn=config")
	if err == nil {
		tlsCert := globalCfg.GetAttributeValue("olcTLSCertificateFile")
		cfg.TLSEnabled = tlsCert != ""
	}

	return cfg, nil
}

// SetACLs replaces the ACL list on the database.
func (c *Client) SetACLs(acls []string) error {
	dn := "olcDatabase={1}mdb,cn=config"
	return c.modifyConfigAttr(dn, "olcAccess", acls)
}

// ConfigureSyncRepl sets up syncrepl on the database for replication.
func (c *Client) ConfigureSyncRepl(cfg *SyncReplConfig) error {
	dn := "olcDatabase={1}mdb,cn=config"

	syncReplValue := fmt.Sprintf(
		"rid=%03d provider=%s type=%s searchbase=\"%s\" binddn=\"%s\" credentials=%s retry=\"60 +\"",
		cfg.RID, cfg.Provider, cfg.Type, cfg.SearchBase, cfg.BindDN, cfg.Credentials,
	)
	if cfg.Interval != "" {
		syncReplValue += fmt.Sprintf(" interval=%s", cfg.Interval)
	}

	return c.modifyConfigAttr(dn, "olcSyncRepl", []string{syncReplValue})
}

// RemoveSyncRepl removes syncrepl configuration.
func (c *Client) RemoveSyncRepl() error {
	dn := "olcDatabase={1}mdb,cn=config"
	req := goldap.NewModifyRequest(dn, nil)
	req.Delete("olcSyncRepl", nil)
	err := c.Modify(req)
	// Ignore error if attribute doesn't exist
	if err != nil && goldap.IsErrorWithCode(err, goldap.LDAPResultNoSuchAttribute) {
		return nil
	}
	return err
}

// EnableSyncProv loads the syncprov overlay for the provider side.
func (c *Client) EnableSyncProv() error {
	// Load module
	modDN := "cn=module{0},cn=config"
	modReq := goldap.NewModifyRequest(modDN, nil)
	modReq.Add("olcModuleLoad", []string{"syncprov"})
	if err := c.Modify(modReq); err != nil {
		if !goldap.IsErrorWithCode(err, goldap.LDAPResultAttributeOrValueExists) {
			return fmt.Errorf("loading syncprov module: %w", err)
		}
	}

	// Add overlay
	overlayDN := "olcOverlay=syncprov,olcDatabase={1}mdb,cn=config"
	addReq := goldap.NewAddRequest(overlayDN, nil)
	addReq.Attribute("objectClass", []string{"olcOverlayConfig", "olcSyncProvConfig"})
	addReq.Attribute("olcOverlay", []string{"syncprov"})
	addReq.Attribute("olcSpCheckpoint", []string{"100 10"})
	addReq.Attribute("olcSpSessionlog", []string{"100"})
	err := c.Add(addReq)
	if err != nil && goldap.IsErrorWithCode(err, goldap.LDAPResultEntryAlreadyExists) {
		return nil
	}
	return err
}

// LoadModule loads an OpenLDAP module via cn=config.
func (c *Client) LoadModule(name string) error {
	dn := "cn=module{0},cn=config"
	req := goldap.NewModifyRequest(dn, nil)
	req.Add("olcModuleLoad", []string{name})
	err := c.Modify(req)
	if err != nil && goldap.IsErrorWithCode(err, goldap.LDAPResultAttributeOrValueExists) {
		return nil
	}
	return err
}

// SetConfigAttr sets a single-valued attribute on cn=config.
func (c *Client) SetConfigAttr(attr, value string) error {
	return c.modifyConfigAttr("cn=config", attr, []string{value})
}

func (c *Client) getConfigEntry(dn string) (*goldap.Entry, error) {
	req := goldap.NewSearchRequest(
		dn,
		goldap.ScopeBaseObject,
		goldap.NeverDerefAliases,
		1, 0, false,
		"(objectClass=*)",
		[]string{"*"},
		nil,
	)
	result, err := c.Search(req)
	if err != nil {
		return nil, err
	}
	if len(result.Entries) == 0 {
		return nil, fmt.Errorf("entry not found: %s", dn)
	}
	return result.Entries[0], nil
}

func (c *Client) modifyConfigAttr(dn, attr string, values []string) error {
	req := goldap.NewModifyRequest(dn, nil)
	req.Replace(attr, values)
	return c.Modify(req)
}

func parseSyncRepl(value string) *SyncReplConfig {
	cfg := &SyncReplConfig{}
	parts := strings.Fields(value)
	for _, p := range parts {
		kv := strings.SplitN(p, "=", 2)
		if len(kv) != 2 {
			continue
		}
		k, v := kv[0], strings.Trim(kv[1], "\"")
		switch k {
		case "rid":
			cfg.RID, _ = strconv.Atoi(v)
		case "provider":
			cfg.Provider = v
		case "searchbase":
			cfg.SearchBase = v
		case "binddn":
			cfg.BindDN = v
		case "type":
			cfg.Type = v
		case "interval":
			cfg.Interval = v
		}
	}
	return cfg
}
