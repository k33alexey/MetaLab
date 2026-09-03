// Package pgcopy creates consistent PostgreSQL database copies with official client tools.
package pgcopy

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

const diagnosticLimit = 64 << 10

var majorVersionPattern = regexp.MustCompile(`PostgreSQL\)\s+(\d+)`)

// Copier locates and runs pg_dump/pg_restore without placing passwords in process arguments.
type Copier struct {
	dumpPath    string
	restorePath string
	run         commandRunner
}

type commandRunner func(context.Context, string, []string, []string, io.Writer) error

// New locates the official PostgreSQL client tools in PATH.
func New() (*Copier, error) {
	dumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return nil, fmt.Errorf("pg_dump was not found in PATH")
	}
	restorePath, err := exec.LookPath("pg_restore")
	if err != nil {
		return nil, fmt.Errorf("pg_restore was not found in PATH")
	}
	return &Copier{dumpPath: dumpPath, restorePath: restorePath, run: runCommand}, nil
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
	if copier == nil || copier.run == nil || copier.dumpPath == "" || copier.restorePath == "" {
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
	toolMajor, err := copier.checkTools(ctx)
	if err != nil {
		return err
	}
	if targetMajor < sourceMajor {
		return fmt.Errorf("target PostgreSQL %d is older than source PostgreSQL %d", targetMajor, sourceMajor)
	}
	if toolMajor < sourceMajor || toolMajor > targetMajor {
		return fmt.Errorf("PostgreSQL client tools %d are incompatible with source %d and target %d", toolMajor, sourceMajor, targetMajor)
	}
	temporaryDirectory, err := os.MkdirTemp("", "metalab-pgcopy-*")
	if err != nil {
		return fmt.Errorf("create PostgreSQL copy directory: %w", err)
	}
	defer os.RemoveAll(temporaryDirectory)
	passfile := filepath.Join(temporaryDirectory, "pgpass")
	passfileContent := pgpassLine(source, sourcePassword) + pgpassLine(target, targetPassword)
	if err := os.WriteFile(passfile, []byte(passfileContent), 0o600); err != nil {
		return fmt.Errorf("create temporary PostgreSQL password file: %w", err)
	}
	if err := protectCredentialFile(passfile); err != nil {
		return err
	}
	archive := filepath.Join(temporaryDirectory, "database.dump")
	environment := commandEnvironment(passfile)
	if err := copier.execute(ctx, copier.dumpPath, []string{
		"--format=custom", "--no-owner", "--no-acl", "--file", archive,
		"--host", source.Host, "--port", strconv.Itoa(int(source.Port)),
		"--username", source.User, "--dbname", source.Database,
	}, environment); err != nil {
		return fmt.Errorf("copy source PostgreSQL database: %w", err)
	}
	if err := copier.execute(ctx, copier.restorePath, []string{
		"--exit-on-error", "--single-transaction", "--no-owner", "--no-acl",
		"--host", target.Host, "--port", strconv.Itoa(int(target.Port)),
		"--username", target.User, "--dbname", target.Database, archive,
	}, environment); err != nil {
		return fmt.Errorf("restore target PostgreSQL database: %w", err)
	}
	return nil
}

func (copier *Copier) checkTools(ctx context.Context) (int, error) {
	dumpMajor, err := copier.toolMajor(ctx, copier.dumpPath)
	if err != nil {
		return 0, err
	}
	restoreMajor, err := copier.toolMajor(ctx, copier.restorePath)
	if err != nil {
		return 0, err
	}
	if dumpMajor != restoreMajor {
		return 0, fmt.Errorf("pg_dump major version %d differs from pg_restore %d", dumpMajor, restoreMajor)
	}
	return dumpMajor, nil
}

func (copier *Copier) toolMajor(ctx context.Context, path string) (int, error) {
	var output bytes.Buffer
	if err := copier.run(ctx, path, []string{"--version"}, commandEnvironment(""), &output); err != nil {
		return 0, fmt.Errorf("check %s version: %w", filepath.Base(path), err)
	}
	match := majorVersionPattern.FindStringSubmatch(output.String())
	if len(match) != 2 {
		return 0, fmt.Errorf("cannot parse %s version", filepath.Base(path))
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return 0, fmt.Errorf("parse %s major version: %w", filepath.Base(path), err)
	}
	return major, nil
}

func (copier *Copier) execute(ctx context.Context, path string, arguments, environment []string) error {
	diagnostics := &limitedBuffer{remaining: diagnosticLimit}
	if err := copier.run(ctx, path, arguments, environment, diagnostics); err != nil {
		message := strings.TrimSpace(diagnostics.String())
		if message == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, message)
	}
	return nil
}

func runCommand(ctx context.Context, path string, arguments, environment []string, output io.Writer) error {
	command := exec.CommandContext(ctx, path, arguments...)
	command.Env = environment
	command.Stdout = output
	command.Stderr = output
	return command.Run()
}

func commandEnvironment(passfile string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "PGPASSWORD=") || strings.HasPrefix(variable, "PGPASSFILE=") || strings.HasPrefix(variable, "PGCONNECT_TIMEOUT=") {
			continue
		}
		environment = append(environment, variable)
	}
	if passfile != "" {
		environment = append(environment, "PGPASSFILE="+passfile)
	}
	return append(environment, "PGCONNECT_TIMEOUT=10")
}

func pgpassLine(descriptor postgresconn.Descriptor, password string) string {
	host := descriptor.Host
	if strings.HasPrefix(host, "/") {
		host = "localhost"
	}
	fields := []string{host, strconv.Itoa(int(descriptor.Port)), descriptor.Database, descriptor.User, password}
	for index := range fields {
		fields[index] = strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(fields[index])
	}
	return strings.Join(fields, ":") + "\n"
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int
}

func (buffer *limitedBuffer) Write(content []byte) (int, error) {
	originalLength := len(content)
	if len(content) > buffer.remaining {
		content = content[:buffer.remaining]
	}
	if len(content) > 0 {
		_, _ = buffer.buffer.Write(content)
		buffer.remaining -= len(content)
	}
	return originalLength, nil
}

func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
