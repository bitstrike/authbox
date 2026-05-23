package db

import (
	"database/sql"
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
