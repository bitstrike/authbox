package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/authbox/authbox/internal/db"
	"github.com/authbox/authbox/internal/logging"
)

const (
	pollInterval    = 10 * time.Second
	snapshotTimeout = 30 * time.Second
	pollTimeout     = 10 * time.Second
)

// ReplicaSync manages the sync loop for a replica container.
type ReplicaSync struct {
	primaryURL  string
	token       string
	repo        *db.Repository
	log         *logging.Logger
	lastVersion int
	client      *http.Client
}

// NewReplicaSync creates a new replica sync manager.
func NewReplicaSync(primaryHost, token string, repo *db.Repository, log *logging.Logger) *ReplicaSync {
	return &ReplicaSync{
		primaryURL: fmt.Sprintf("https://%s:8443", primaryHost),
		token:      token,
		repo:       repo,
		log:        log,
		client:     &http.Client{},
	}
}

// Start begins the sync loop. It fetches a full snapshot on startup,
// then polls for deltas.
func (rs *ReplicaSync) Start(ctx context.Context) {
	rs.log.Info("replica sync starting", "primary", rs.primaryURL)

	// Initial snapshot
	if err := rs.fetchSnapshot(ctx); err != nil {
		rs.log.Error("initial snapshot failed", "err", err)
	}

	// Poll loop
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			rs.log.Info("replica sync stopped")
			return
		case <-ticker.C:
			if err := rs.pollChanges(ctx); err != nil {
				rs.log.Error("poll failed", "err", err)
			}
		}
	}
}

func (rs *ReplicaSync) fetchSnapshot(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, snapshotTimeout)
	defer cancel()

	body, err := rs.doRequest(ctx, "/internal/sync/snapshot")
	if err != nil {
		return err
	}
	defer body.Close()

	var snapshot db.Snapshot
	if err := json.NewDecoder(body).Decode(&snapshot); err != nil {
		return fmt.Errorf("decoding snapshot: %w", err)
	}

	// Restore state
	for i := range snapshot.FIDO2 {
		rs.repo.CreateFIDO2Credential(&snapshot.FIDO2[i])
	}
	for i := range snapshot.ServiceAccounts {
		rs.repo.CreateServiceAccount(&snapshot.ServiceAccounts[i])
	}

	rs.lastVersion = snapshot.Version
	rs.log.Info("snapshot applied", "version", rs.lastVersion)
	return nil
}

func (rs *ReplicaSync) pollChanges(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, pollTimeout)
	defer cancel()

	url := fmt.Sprintf("/internal/sync/changes?since=%d", rs.lastVersion)
	body, err := rs.doRequest(ctx, url)
	if err != nil {
		return err
	}
	defer body.Close()

	var resp struct {
		Changes []db.SyncEntry `json:"changes"`
		Count   int            `json:"count"`
	}
	if err := json.NewDecoder(body).Decode(&resp); err != nil {
		return fmt.Errorf("decoding changes: %w", err)
	}

	if resp.Count == 0 {
		return nil
	}

	// Apply changes
	for _, entry := range resp.Changes {
		if err := rs.applyChange(entry); err != nil {
			rs.log.Error("apply change failed", "version", entry.Version, "err", err)
			// If we fall too far behind, re-snapshot
			return rs.fetchSnapshot(ctx)
		}
		rs.lastVersion = entry.Version
	}

	rs.log.Info("changes applied", "count", resp.Count, "version", rs.lastVersion)
	return nil
}

func (rs *ReplicaSync) applyChange(entry db.SyncEntry) error {
	switch entry.TableName {
	case "fido2_credentials":
		return rs.applyFIDO2Change(entry)
	case "service_accounts":
		return rs.applyServiceAccountChange(entry)
	case "ssh_certs":
		return rs.applySSHCertChange(entry)
	}
	return nil
}

func (rs *ReplicaSync) applyFIDO2Change(entry db.SyncEntry) error {
	switch entry.Operation {
	case "insert":
		var cred db.FIDO2Credential
		if err := json.Unmarshal([]byte(entry.Data), &cred); err != nil {
			return err
		}
		return rs.repo.CreateFIDO2Credential(&cred)
	case "delete":
		return rs.repo.DeleteFIDO2CredentialByID(entry.RowID)
	}
	return nil
}

func (rs *ReplicaSync) applyServiceAccountChange(entry db.SyncEntry) error {
	switch entry.Operation {
	case "insert":
		var sa db.ServiceAccount
		if err := json.Unmarshal([]byte(entry.Data), &sa); err != nil {
			return err
		}
		return rs.repo.CreateServiceAccount(&sa)
	case "delete":
		return rs.repo.DeleteServiceAccount(entry.Data)
	}
	return nil
}

func (rs *ReplicaSync) applySSHCertChange(entry db.SyncEntry) error {
	if entry.Operation == "insert" {
		var cert db.SSHCert
		if err := json.Unmarshal([]byte(entry.Data), &cert); err != nil {
			return err
		}
		return rs.repo.CreateSSHCert(&cert)
	}
	return nil
}

func (rs *ReplicaSync) doRequest(ctx context.Context, path string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rs.primaryURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Internal-Token", rs.token)

	resp, err := rs.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", path, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("request %s: status %d", path, resp.StatusCode)
	}
	return resp.Body, nil
}
