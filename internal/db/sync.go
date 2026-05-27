// sync.go implements the sync log for primary-to-replica replication. Records
// changes (insert/update/delete) with version numbers, provides GetSyncState,
// GetChangesSince for incremental polling, and GetSnapshot for full initial
// sync. Used by the replica sync loop and the /internal/sync/* API endpoints.
package db

import (
	"encoding/json"
	"time"
)

// SyncEntry represents a single change in the sync log.
type SyncEntry struct {
	ID        int       `json:"id"`
	Version   int       `json:"version"`
	TableName string    `json:"table_name"`
	Operation string    `json:"operation"` // insert, update, delete
	RowID     int       `json:"row_id"`
	Data      string    `json:"data,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// SyncState holds the current sync version.
type SyncState struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Snapshot holds the full database state for initial sync.
type Snapshot struct {
	Version         int                `json:"version"`
	FIDO2           []FIDO2Credential  `json:"fido2_credentials"`
	ServiceAccounts []ServiceAccount   `json:"service_accounts"`
	SSHCerts        []SSHCert          `json:"ssh_certs"`
	CreatedAt       time.Time          `json:"created_at"`
}

// GetSyncState returns the current highest version number.
func (r *Repository) GetSyncState() (*SyncState, error) {
	var version int
	var updatedAt time.Time
	err := r.db.QueryRow("SELECT COALESCE(MAX(version), 0), COALESCE(MAX(created_at), CURRENT_TIMESTAMP) FROM sync_log").
		Scan(&version, &updatedAt)
	if err != nil {
		return &SyncState{Version: 0, UpdatedAt: time.Now()}, nil
	}
	return &SyncState{Version: version, UpdatedAt: updatedAt}, nil
}

// GetChangesSince returns all sync log entries after the given version.
func (r *Repository) GetChangesSince(version int) ([]SyncEntry, error) {
	rows, err := r.db.Query(
		"SELECT id, version, table_name, operation, row_id, data, created_at FROM sync_log WHERE version > ? ORDER BY version ASC",
		version,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []SyncEntry
	for rows.Next() {
		var e SyncEntry
		var data *string
		if err := rows.Scan(&e.ID, &e.Version, &e.TableName, &e.Operation, &e.RowID, &data, &e.CreatedAt); err != nil {
			return nil, err
		}
		if data != nil {
			e.Data = *data
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// GetSnapshot returns the full database state for initial replica sync.
func (r *Repository) GetSnapshot() (*Snapshot, error) {
	state, err := r.GetSyncState()
	if err != nil {
		return nil, err
	}

	fido2, err := r.GetAllFIDO2Credentials()
	if err != nil {
		return nil, err
	}

	accounts, err := r.ListServiceAccounts()
	if err != nil {
		return nil, err
	}

	certs, _, err := r.ListSSHCerts(0, 100000)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		Version:         state.Version,
		FIDO2:           fido2,
		ServiceAccounts: accounts,
		SSHCerts:        certs,
		CreatedAt:       time.Now(),
	}, nil
}

// LogChange records a change in the sync log for replication.
func (r *Repository) LogChange(tableName, operation string, rowID int, data any) error {
	state, _ := r.GetSyncState()
	nextVersion := state.Version + 1

	var dataStr string
	if data != nil {
		b, _ := json.Marshal(data)
		dataStr = string(b)
	}

	_, err := r.db.Exec(
		"INSERT INTO sync_log (version, table_name, operation, row_id, data) VALUES (?, ?, ?, ?, ?)",
		nextVersion, tableName, operation, rowID, dataStr,
	)
	return err
}
