// backup.go implements full platform export and import. Export produces a
// gzipped tar containing LDAP directory LDIF (via slapcat), cn=config LDIF,
// and application state (FIDO2 credentials, service accounts, SSH certs) as
// JSON. Import parses the archive and restores via slapadd and SQLite inserts.
// Also provides scheduled backup and old backup cleanup.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/authbox/authbox/internal/constants"
	"github.com/authbox/authbox/internal/db"
)

// Export holds the metadata for an export archive.
type Export struct {
	Version   int       `json:"version"`
	CreatedAt time.Time `json:"created_at"`
	Hostname  string    `json:"hostname"`
}

// ExportData contains all exportable application state.
type ExportData struct {
	Meta            Export                `json:"meta"`
	FIDO2           []db.FIDO2Credential  `json:"fido2_credentials"`
	ServiceAccounts []db.ServiceAccount   `json:"service_accounts"`
	SSHCerts        []db.SSHCert          `json:"ssh_certs"`
}

const (
	exportVersion    = 1
	metaFile         = "meta.json"
	directoryLDIF    = "directory.ldif"
	configLDIF       = "config.ldif"
	appStateFile     = "state.json"
)

// CreateExport generates a gzipped tar archive containing the full platform state.
func CreateExport(w io.Writer, repo *db.Repository, slapcatPath string) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	hostname, _ := os.Hostname()
	meta := Export{
		Version:   exportVersion,
		CreatedAt: time.Now(),
		Hostname:  hostname,
	}

	// Dump LDAP directory
	dirLDIF, err := runSlapcat(slapcatPath, "")
	if err != nil {
		return fmt.Errorf("slapcat directory: %w", err)
	}
	if err := addToTar(tw, directoryLDIF, dirLDIF); err != nil {
		return err
	}

	// Dump cn=config
	cfgLDIF, err := runSlapcat(slapcatPath, "cn=config")
	if err != nil {
		return fmt.Errorf("slapcat config: %w", err)
	}
	if err := addToTar(tw, configLDIF, cfgLDIF); err != nil {
		return err
	}

	// Export application state from SQLite
	state, err := exportAppState(repo, meta)
	if err != nil {
		return fmt.Errorf("export app state: %w", err)
	}
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := addToTar(tw, appStateFile, stateJSON); err != nil {
		return err
	}

	// Meta file
	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	return addToTar(tw, metaFile, metaJSON)
}

// ImportExport reads a gzipped tar archive and returns the parsed contents.
func ImportExport(r io.Reader) (*ImportData, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	data := &ImportData{}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading tar: %w", err)
		}

		content, err := io.ReadAll(tr)
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", hdr.Name, err)
		}

		switch hdr.Name {
		case metaFile:
			json.Unmarshal(content, &data.Meta)
		case directoryLDIF:
			data.DirectoryLDIF = content
		case configLDIF:
			data.ConfigLDIF = content
		case appStateFile:
			json.Unmarshal(content, &data.State)
		}
	}

	if data.Meta.Version == 0 {
		return nil, fmt.Errorf("invalid export: missing meta")
	}
	return data, nil
}

// ImportData holds parsed contents from an export archive.
type ImportData struct {
	Meta          Export
	DirectoryLDIF []byte
	ConfigLDIF    []byte
	State         ExportData
}

// RestoreState imports application state into the database.
func RestoreState(repo *db.Repository, state *ExportData) error {
	for i := range state.FIDO2 {
		if err := repo.CreateFIDO2Credential(&state.FIDO2[i]); err != nil {
			return fmt.Errorf("restoring fido2 credential: %w", err)
		}
	}
	for i := range state.ServiceAccounts {
		if err := repo.CreateServiceAccount(&state.ServiceAccounts[i]); err != nil {
			return fmt.Errorf("restoring service account: %w", err)
		}
	}
	return nil
}

// RestoreLDAP runs slapadd to restore LDAP data from LDIF.
func RestoreLDAP(slapaddPath string, ldifData []byte, configDB string) error {
	if len(ldifData) == 0 {
		return nil
	}

	tmpFile, err := os.CreateTemp("", "authbox-restore-*.ldif")
	if err != nil {
		return err
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(ldifData); err != nil {
		tmpFile.Close()
		return err
	}
	tmpFile.Close()

	args := []string{"-l", tmpFile.Name()}
	if configDB != "" {
		args = append(args, "-b", configDB)
	}

	cmd := exec.Command(slapaddPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("slapadd: %s: %w", string(output), err)
	}
	return nil
}

// ScheduledBackup runs a slapcat backup to the specified directory.
func ScheduledBackup(slapcatPath, backupDir string, retentionDays int) error {
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return err
	}

	timestamp := time.Now().Format(constants.TimeFormatDate)
	filename := filepath.Join(backupDir, fmt.Sprintf("backup-%s.ldif", timestamp))

	data, err := runSlapcat(slapcatPath, "")
	if err != nil {
		return err
	}

	if err := os.WriteFile(filename, data, 0640); err != nil {
		return err
	}

	// Clean old backups
	return CleanOldBackups(backupDir, retentionDays)
}

// CreateExportFromState generates an export archive from app state only (no slapcat).
// Used for testing and environments without slapd.
func CreateExportFromState(w io.Writer, repo *db.Repository) error {
	gz := gzip.NewWriter(w)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	hostname, _ := os.Hostname()
	meta := Export{
		Version:   exportVersion,
		CreatedAt: time.Now(),
		Hostname:  hostname,
	}

	state, err := exportAppState(repo, meta)
	if err != nil {
		return err
	}
	stateJSON, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	if err := addToTar(tw, appStateFile, stateJSON); err != nil {
		return err
	}

	metaJSON, _ := json.MarshalIndent(meta, "", "  ")
	return addToTar(tw, metaFile, metaJSON)
}

func runSlapcat(slapcatPath, base string) ([]byte, error) {
	args := []string{}
	if base != "" {
		args = append(args, "-b", base)
	}
	cmd := exec.Command(slapcatPath, args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("slapcat %v: %w", args, err)
	}
	return output, nil
}

func exportAppState(repo *db.Repository, meta Export) (*ExportData, error) {
	fido2, err := repo.GetAllFIDO2Credentials()
	if err != nil {
		return nil, err
	}

	accounts, err := repo.ListServiceAccounts()
	if err != nil {
		return nil, err
	}

	certs, _, err := repo.ListSSHCerts(0, 100000)
	if err != nil {
		return nil, err
	}

	return &ExportData{
		Meta:            meta,
		FIDO2:           fido2,
		ServiceAccounts: accounts,
		SSHCerts:        certs,
	}, nil
}

func addToTar(tw *tar.Writer, name string, data []byte) error {
	hdr := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0640,
		ModTime: time.Now(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	_, err := tw.Write(data)
	return err
}

func CleanOldBackups(dir string, retentionDays int) error {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			os.Remove(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}
