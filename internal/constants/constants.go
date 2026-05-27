// constants.go defines shared string constants used across the application:
// shell paths, filesystem prefixes, confirmation words, time formats, frontend
// route paths, and numeric defaults. Intended to replace scattered hardcoded
// literals throughout the codebase.
package constants

// Shells
const (
	ShellBash   = "/bin/bash"
	ShellNologin = "/sbin/nologin"
)

// Filesystem paths
const (
	HomeDirPrefix = "/home/"
)

// Confirmation
const (
	ConfirmWord = "yesiagree"
)

// Time formats
const (
	TimeFormatDateTime = "2006-01-02 15:04"
	TimeFormatDate     = "2006-01-02"
)

// Frontend routes
const (
	RouteLogin           = "/login"
	RouteUsers           = "/users"
	RouteGroups          = "/groups"
	RouteFIDO2           = "/fido2"
	RouteSSH             = "/ssh"
	RouteServiceAccounts = "/service-accounts"
	RouteLogs            = "/logs"
	RouteSettings        = "/settings"
	RouteBackup          = "/backup"
	RouteStatus          = "/status"
)

// Defaults
const (
	DefaultSSHCertTTLSeconds uint64 = 43200 // 12 hours
	DefaultLogRetentionDays         = 90
	DefaultSessionTimeoutMin        = 30
	DefaultBackupTime               = "02:00"
	DefaultBackupRetentionDays      = 30
)
