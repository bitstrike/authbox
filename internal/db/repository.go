// repository.go defines the data model structs (ServiceAccount, SSHCert,
// FIDO2Credential) and the Repository type that provides CRUD operations
// against SQLite for all application state: service accounts, SSH certificates,
// and FIDO2 credentials.
package db

import (
	"database/sql"
	"fmt"
	"time"
)

type ServiceAccount struct {
	ID               int
	ClientID         string
	ClientSecretHash string
	Description      string
	Role             string
	CreatedAt        time.Time
	LastUsedAt       *time.Time
}

type SSHCert struct {
	ID        int
	Username  string
	Serial    string
	Principal string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

type FIDO2Credential struct {
	ID             int
	UID            string
	CredentialData string
	RegisteredAt   time.Time
}

type Repository struct {
	db *sql.DB
}

func NewRepository(database *DB) *Repository {
	return &Repository{db: database.Conn()}
}

func (r *Repository) CreateServiceAccount(sa *ServiceAccount) error {
	_, err := r.db.Exec(
		"INSERT INTO service_accounts (client_id, client_secret_hash, description, role) VALUES (?, ?, ?, ?)",
		sa.ClientID, sa.ClientSecretHash, sa.Description, sa.Role,
	)
	return err
}

func (r *Repository) CreateSSHCert(cert *SSHCert) error {
	_, err := r.db.Exec(
		"INSERT INTO ssh_certs (username, serial, principal, expires_at) VALUES (?, ?, ?, ?)",
		cert.Username, cert.Serial, cert.Principal, cert.ExpiresAt,
	)
	return err
}

func (r *Repository) CreateFIDO2Credential(cred *FIDO2Credential) error {
	_, err := r.db.Exec(
		"INSERT INTO fido2_credentials (uid, credential_data) VALUES (?, ?)",
		cred.UID, cred.CredentialData,
	)
	return err
}

func (r *Repository) GetFIDO2Credentials(uid string) ([]FIDO2Credential, error) {
	rows, err := r.db.Query("SELECT id, uid, credential_data, registered_at FROM fido2_credentials WHERE uid = ?", uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []FIDO2Credential
	for rows.Next() {
		var c FIDO2Credential
		if err := rows.Scan(&c.ID, &c.UID, &c.CredentialData, &c.RegisteredAt); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (r *Repository) GetAllFIDO2Credentials() ([]FIDO2Credential, error) {
	rows, err := r.db.Query("SELECT id, uid, credential_data, registered_at FROM fido2_credentials")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []FIDO2Credential
	for rows.Next() {
		var c FIDO2Credential
		if err := rows.Scan(&c.ID, &c.UID, &c.CredentialData, &c.RegisteredAt); err != nil {
			return nil, err
		}
		creds = append(creds, c)
	}
	return creds, rows.Err()
}

func (r *Repository) DeleteFIDO2Credentials(uid string) error {
	_, err := r.db.Exec("DELETE FROM fido2_credentials WHERE uid = ?", uid)
	return err
}

func (r *Repository) GetFIDO2CredentialByID(id int) (*FIDO2Credential, error) {
	var c FIDO2Credential
	err := r.db.QueryRow("SELECT id, uid, credential_data, registered_at FROM fido2_credentials WHERE id = ?", id).
		Scan(&c.ID, &c.UID, &c.CredentialData, &c.RegisteredAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) DeleteFIDO2CredentialByID(id int) error {
	_, err := r.db.Exec("DELETE FROM fido2_credentials WHERE id = ?", id)
	return err
}

func (r *Repository) ListServiceAccounts() ([]ServiceAccount, error) {
	rows, err := r.db.Query("SELECT id, client_id, description, role, created_at, last_used_at FROM service_accounts")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []ServiceAccount
	for rows.Next() {
		var sa ServiceAccount
		if err := rows.Scan(&sa.ID, &sa.ClientID, &sa.Description, &sa.Role, &sa.CreatedAt, &sa.LastUsedAt); err != nil {
			return nil, err
		}
		accounts = append(accounts, sa)
	}
	return accounts, rows.Err()
}

func (r *Repository) GetServiceAccountByClientID(clientID string) (*ServiceAccount, error) {
	var sa ServiceAccount
	err := r.db.QueryRow(
		"SELECT id, client_id, client_secret_hash, description, role, created_at, last_used_at FROM service_accounts WHERE client_id = ?",
		clientID,
	).Scan(&sa.ID, &sa.ClientID, &sa.ClientSecretHash, &sa.Description, &sa.Role, &sa.CreatedAt, &sa.LastUsedAt)
	if err != nil {
		return nil, err
	}
	return &sa, nil
}

func (r *Repository) DeleteServiceAccount(clientID string) error {
	_, err := r.db.Exec("DELETE FROM service_accounts WHERE client_id = ?", clientID)
	return err
}

func (r *Repository) UpdateServiceAccountLastUsed(clientID string) {
	r.db.Exec("UPDATE service_accounts SET last_used_at = CURRENT_TIMESTAMP WHERE client_id = ?", clientID)
}

func (r *Repository) ListSSHCerts(offset, limit int) ([]SSHCert, int, error) {
	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM ssh_certs").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(
		"SELECT id, username, serial, principal, issued_at, expires_at FROM ssh_certs ORDER BY issued_at DESC LIMIT ? OFFSET ?",
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var certs []SSHCert
	for rows.Next() {
		var c SSHCert
		if err := rows.Scan(&c.ID, &c.Username, &c.Serial, &c.Principal, &c.IssuedAt, &c.ExpiresAt); err != nil {
			return nil, 0, err
		}
		certs = append(certs, c)
	}
	return certs, total, rows.Err()
}

// ListSSHCertsSorted returns certs with configurable sort column and order.
func (r *Repository) ListSSHCertsSorted(offset, limit int, sortCol, sortOrder string) ([]SSHCert, int, error) {
	// Whitelist sort columns to prevent SQL injection
	allowedCols := map[string]bool{
		"username": true, "serial": true, "issued_at": true, "expires_at": true,
	}
	if !allowedCols[sortCol] {
		sortCol = "issued_at"
	}
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc"
	}

	var total int
	err := r.db.QueryRow("SELECT COUNT(*) FROM ssh_certs").Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := fmt.Sprintf(
		"SELECT id, username, serial, principal, issued_at, expires_at FROM ssh_certs ORDER BY %s %s LIMIT ? OFFSET ?",
		sortCol, sortOrder,
	)
	rows, err := r.db.Query(query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var certs []SSHCert
	for rows.Next() {
		var c SSHCert
		if err := rows.Scan(&c.ID, &c.Username, &c.Serial, &c.Principal, &c.IssuedAt, &c.ExpiresAt); err != nil {
			return nil, 0, err
		}
		certs = append(certs, c)
	}
	return certs, total, rows.Err()
}

// CleanExpiredCerts removes cert records that expired more than retentionDays ago.
func (r *Repository) CleanExpiredCerts(retentionDays int) (int64, error) {
	result, err := r.db.Exec(
		"DELETE FROM ssh_certs WHERE expires_at < datetime('now', ? || ' days')",
		fmt.Sprintf("-%d", retentionDays),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
