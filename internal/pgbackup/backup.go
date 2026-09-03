// Package pgbackup creates and atomically restores verified local PostgreSQL archives.
package pgbackup

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/postgresconn"
)

const diagnosticLimit = 64 << 10

// Archive is the completed local backup metadata.
type Archive struct {
	FileName  string
	SizeBytes int64
	SHA256    string
}

// Tool runs official PostgreSQL client utilities.
type Tool struct {
	dumpPath    string
	restorePath string
	run         commandRunner
}

type commandRunner func(context.Context, string, []string, []string, io.Writer) error

// New locates pg_dump and pg_restore.
func New() (*Tool, error) {
	dumpPath, err := exec.LookPath("pg_dump")
	if err != nil {
		return nil, fmt.Errorf("pg_dump was not found in PATH")
	}
	restorePath, err := exec.LookPath("pg_restore")
	if err != nil {
		return nil, fmt.Errorf("pg_restore was not found in PATH")
	}
	return &Tool{dumpPath: dumpPath, restorePath: restorePath, run: runCommand}, nil
}

// Create writes a custom-format archive through a protected temporary file and atomic rename.
func (tool *Tool) Create(ctx context.Context, connection postgresconn.Descriptor, password, directory, fileName string) (Archive, error) {
	if err := validateInputs(tool, connection, password, directory, fileName); err != nil {
		return Archive{}, err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return Archive{}, fmt.Errorf("create backup directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".backup-*.partial")
	if err != nil {
		return Archive{}, fmt.Errorf("create temporary backup: %w", err)
	}
	temporaryPath := temporary.Name()
	if err := temporary.Close(); err != nil {
		return Archive{}, fmt.Errorf("close temporary backup: %w", err)
	}
	defer os.Remove(temporaryPath)
	if err := protectFile(temporaryPath); err != nil {
		return Archive{}, fmt.Errorf("protect temporary backup: %w", err)
	}
	err = withPasswordFile(directory, connection, password, func(environment []string) error {
		return tool.execute(ctx, tool.dumpPath, []string{
			"--format=custom", "--no-owner", "--no-acl", "--file", temporaryPath,
			"--host", connection.Host, "--port", strconv.Itoa(int(connection.Port)),
			"--username", connection.User, "--dbname", connection.Database,
		}, environment)
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
	if err := os.Rename(temporaryPath, target); err != nil {
		return Archive{}, fmt.Errorf("publish backup: %w", err)
	}
	return Archive{FileName: fileName, SizeBytes: size, SHA256: hash}, nil
}

// Restore verifies the archive and restores it transactionally into its registered database.
func (tool *Tool) Restore(ctx context.Context, connection postgresconn.Descriptor, password, path, expectedSHA256 string) error {
	if tool == nil || tool.run == nil || tool.restorePath == "" || password == "" {
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
	directory := filepath.Dir(path)
	return withPasswordFile(directory, connection, password, func(environment []string) error {
		if err := tool.execute(ctx, tool.restorePath, []string{
			"--clean", "--if-exists", "--exit-on-error", "--single-transaction", "--no-owner", "--no-acl",
			"--host", connection.Host, "--port", strconv.Itoa(int(connection.Port)),
			"--username", connection.User, "--dbname", connection.Database, path,
		}, environment); err != nil {
			return fmt.Errorf("restore PostgreSQL backup: %w", err)
		}
		return nil
	})
}

func validateInputs(tool *Tool, connection postgresconn.Descriptor, password, directory, fileName string) error {
	if tool == nil || tool.run == nil || tool.dumpPath == "" || password == "" {
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

func withPasswordFile(directory string, connection postgresconn.Descriptor, password string, action func([]string) error) error {
	file, err := os.CreateTemp(directory, ".pgpass-*")
	if err != nil {
		return fmt.Errorf("create temporary PostgreSQL password file: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	host := connection.Host
	if strings.HasPrefix(host, "/") {
		host = "localhost"
	}
	escape := func(value string) string { return strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(value) }
	line := strings.Join([]string{
		escape(host), strconv.Itoa(int(connection.Port)), escape(connection.Database), escape(connection.User), escape(password),
	}, ":") + "\n"
	if _, err := file.WriteString(line); err != nil {
		_ = file.Close()
		return fmt.Errorf("write temporary PostgreSQL password file: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close temporary PostgreSQL password file: %w", err)
	}
	if err := protectFile(path); err != nil {
		return err
	}
	return action(commandEnvironment(path))
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

func (tool *Tool) execute(ctx context.Context, path string, arguments, environment []string) error {
	diagnostics := &limitedBuffer{remaining: diagnosticLimit}
	if err := tool.run(ctx, path, arguments, environment, diagnostics); err != nil {
		message := strings.TrimSpace(diagnostics.String())
		if message != "" {
			return fmt.Errorf("%w: %s", err, message)
		}
		return err
	}
	return nil
}

func runCommand(ctx context.Context, path string, arguments, environment []string, output io.Writer) error {
	command := exec.CommandContext(ctx, path, arguments...)
	command.Env, command.Stdout, command.Stderr = environment, output, output
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
	return append(environment, "PGPASSFILE="+passfile, "PGCONNECT_TIMEOUT=10")
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	remaining int
}

func (buffer *limitedBuffer) Write(content []byte) (int, error) {
	length := len(content)
	if len(content) > buffer.remaining {
		content = content[:buffer.remaining]
	}
	_, _ = buffer.buffer.Write(content)
	buffer.remaining -= len(content)
	return length, nil
}

func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
