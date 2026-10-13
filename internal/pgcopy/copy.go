// Package pgcopy creates consistent PostgreSQL database copies with official client tools.
package pgcopy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k33alexey/MetaLab/internal/ownerfile"
	"github.com/k33alexey/MetaLab/internal/pgtool"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

// Copier runs pg_dump/pg_restore without placing passwords in process arguments.
type Copier struct {
	tools pgtool.Tools
}

// New locates the official PostgreSQL client tools in PATH.
func New() (*Copier, error) {
	tools, err := pgtool.Locate()
	if err != nil {
		return nil, err
	}
	return &Copier{tools: tools}, nil
}

// Copy creates and restores a consistent custom-format archive without stopping the source.
func (copier *Copier) Copy(
	ctx context.Context,
	source postgresconn.Descriptor,
	sourcePassword string,
	sourceMajor int,
	target postgresconn.Descriptor,
	targetPassword string,
	targetMajor int,
) error {
	if copier == nil || !copier.tools.Ready() {
		return fmt.Errorf("PostgreSQL copy tools are not configured")
	}
	if sourcePassword == "" || targetPassword == "" {
		return fmt.Errorf("source and target PostgreSQL passwords are required")
	}
	if err := source.Validate(); err != nil {
		return fmt.Errorf("invalid source connection: %w", err)
	}
	if err := target.Validate(); err != nil {
		return fmt.Errorf("invalid target connection: %w", err)
	}
	toolMajor, err := copier.tools.Major(ctx)
	if err != nil {
		return err
	}
	if targetMajor < sourceMajor {
		return fmt.Errorf("target PostgreSQL %d is older than source PostgreSQL %d", targetMajor, sourceMajor)
	}
	if toolMajor < sourceMajor || toolMajor > targetMajor {
		return fmt.Errorf("PostgreSQL client tools %d are incompatible with source %d and target %d", toolMajor, sourceMajor, targetMajor)
	}
	temporaryDirectory, err := ownerfile.MkdirTemp("", "metalab-pgcopy-*")
	if err != nil {
		return fmt.Errorf("create PostgreSQL copy directory: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)
	archive := filepath.Join(temporaryDirectory, "database.dump")
	credentials := []postgresconn.Credential{
		{Connection: source, Password: sourcePassword},
		{Connection: target, Password: targetPassword},
	}
	return postgresconn.WithPasswordFile(credentials, func(environment []string) error {
		if err := copier.tools.Dump(ctx, source, archive, environment); err != nil {
			return fmt.Errorf("copy source PostgreSQL database: %w", err)
		}
		if err := copier.tools.Restore(ctx, target, archive, pgtool.IntoEmpty, environment); err != nil {
			return fmt.Errorf("restore target PostgreSQL database: %w", err)
		}
		return nil
	})
}
