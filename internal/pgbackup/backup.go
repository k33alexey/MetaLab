// Package pgbackup creates and atomically restores verified local PostgreSQL archives.
package pgbackup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/k33alexey/MetaLab/internal/ownerfile"
	"github.com/k33alexey/MetaLab/internal/pgtool"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/servicestate"
)

// Archive is the completed local backup metadata.
type Archive struct {
	FileName  string
	SizeBytes int64
	SHA256    string
}

// Tool runs official PostgreSQL client utilities.
type Tool struct {
	tools pgtool.Tools
}

// New locates pg_dump and pg_restore.
func New() (*Tool, error) {
	tools, err := pgtool.Locate()
	if err != nil {
		return nil, err
	}
	return &Tool{tools: tools}, nil
}

// Create writes a custom-format archive into an existing directory through a
// protected temporary file and atomic rename.
func (tool *Tool) Create(ctx context.Context, connection postgresconn.Descriptor, password, directory, fileName string) (Archive, error) {
	if err := validateInputs(tool, connection, password, directory, fileName); err != nil {
		return Archive{}, err
	}
	temporary, err := ownerfile.CreateTemp(directory, ".backup-*.partial")
	if err != nil {
		return Archive{}, fmt.Errorf("create temporary backup: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Close(); err != nil {
		return Archive{}, fmt.Errorf("close temporary backup: %w", err)
	}
	credentials := []postgresconn.Credential{{Connection: connection, Password: password}}
	err = postgresconn.WithPasswordFile(credentials, func(environment []string) error {
		return tool.tools.Dump(ctx, connection, temporaryPath, environment)
	})
	if err != nil {
		return Archive{}, fmt.Errorf("create PostgreSQL backup: %w", err)
	}
	hash, size, err := fileDigest(temporaryPath)
	if err != nil {
		return Archive{}, err
	}
	target := filepath.Join(directory, fileName)
	if _, err := os.Stat(target); err == nil {
		return Archive{}, fmt.Errorf("backup file %q already exists", fileName)
	} else if !os.IsNotExist(err) {
		return Archive{}, fmt.Errorf("inspect backup target: %w", err)
	}
	// The service restores what an administrator archived, so the archive is
	// handed to it before it gets its name, as a secret is.
	if err := servicestate.HandOver(temporaryPath); err != nil {
		return Archive{}, err
	}
	if err := os.Rename(temporaryPath, target); err != nil {
		return Archive{}, fmt.Errorf("publish backup: %w", err)
	}
	return Archive{FileName: fileName, SizeBytes: size, SHA256: hash}, nil
}

// Restore verifies the archive and restores it transactionally into its registered database.
func (tool *Tool) Restore(ctx context.Context, connection postgresconn.Descriptor, password, path, expectedSHA256 string) error {
	if tool == nil || !tool.tools.Ready() || password == "" {
		return fmt.Errorf("PostgreSQL restore tool is not configured")
	}
	if err := connection.Validate(); err != nil {
		return err
	}
	actual, _, err := fileDigest(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(actual, expectedSHA256) {
		return fmt.Errorf("backup checksum mismatch")
	}
	credentials := []postgresconn.Credential{{Connection: connection, Password: password}}
	return postgresconn.WithPasswordFile(credentials, func(environment []string) error {
		if err := tool.tools.Restore(ctx, connection, path, pgtool.ReplaceExisting, environment); err != nil {
			return fmt.Errorf("restore PostgreSQL backup: %w", err)
		}
		return nil
	})
}

func validateInputs(tool *Tool, connection postgresconn.Descriptor, password, directory, fileName string) error {
	if tool == nil || !tool.tools.Ready() || password == "" {
		return fmt.Errorf("PostgreSQL backup tool is not configured")
	}
	if err := connection.Validate(); err != nil {
		return err
	}
	if directory == "" || !filepath.IsAbs(directory) {
		return fmt.Errorf("backup directory must be absolute")
	}
	if filepath.Base(fileName) != fileName || !strings.HasSuffix(fileName, ".mlbackup") {
		return fmt.Errorf("invalid backup file name")
	}
	return nil
}

func fileDigest(path string) (string, int64, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", 0, fmt.Errorf("open backup: %w", err)
	}
	defer file.Close()
	hash := sha256.New()
	size, err := io.Copy(hash, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash backup: %w", err)
	}
	return hex.EncodeToString(hash.Sum(nil)), size, nil
}
