package postgresconn

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/filelock"
	"github.com/k33alexey/MetaLab/internal/ownerfile"
)

// Credential is a connection with the password a PostgreSQL client tool uses for it.
type Credential struct {
	Connection Descriptor
	Password   string
}

// Password directories are named by passwordDirectoryPattern in the system
// temporary directory; the lock file in one says whether its process lives.
const (
	passwordDirectoryPattern = "metalab-pgpass-*"
	passwordLockName         = "lock"
)

// WithPasswordFile hands passwords to PostgreSQL client tools (pg_dump,
// pg_restore) through a password file, never through arguments or
// PGPASSWORD, which other users of the machine may see. The file lies in a
// new owner-only temporary directory, not beside the data the tool works on,
// and is created closed before the passwords are written. The directory is
// removed when action returns; action receives the tool environment. While
// it exists the directory holds its lock, so a process killed before removing
// it leaves it to RemoveAbandonedPasswordFiles.
func WithPasswordFile(credentials []Credential, action func(environment []string) error) error {
	directory, err := ownerfile.MkdirTemp("", passwordDirectoryPattern)
	if err != nil {
		return fmt.Errorf("create PostgreSQL password directory: %w", err)
	}
	defer os.RemoveAll(directory)
	held, err := holdPasswordDirectory(directory)
	if err != nil {
		return err
	}
	// Released before the directory is removed: Windows keeps a directory
	// with an open file in it.
	defer held.Release()
	path := filepath.Join(directory, "pgpass")
	file, err := ownerfile.Create(path)
	if err != nil {
		return fmt.Errorf("create temporary PostgreSQL password file: %w", err)
	}
	var content strings.Builder
	for _, credential := range credentials {
		content.WriteString(passwordLine(credential))
	}
	_, err = file.WriteString(content.String())
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write temporary PostgreSQL password file: %w", err)
	}
	return action(ToolEnvironment(path))
}

// holdPasswordDirectory locks the lock file of a new password directory
// before any password is written to it. The file is locked under another name
// and only then named lock, so a lock file that can be taken never belongs to
// a living process.
func holdPasswordDirectory(directory string) (*filelock.Lock, error) {
	pending := filepath.Join(directory, passwordLockName+".pending")
	file, err := ownerfile.Create(pending)
	if err != nil {
		return nil, fmt.Errorf("create PostgreSQL password directory lock: %w", err)
	}
	held, err := filelock.Hold(file)
	if err != nil {
		return nil, fmt.Errorf("lock PostgreSQL password directory: %w", err)
	}
	if err := os.Rename(pending, filepath.Join(directory, passwordLockName)); err != nil {
		held.Release()
		return nil, fmt.Errorf("lock PostgreSQL password directory: %w", err)
	}
	return held, nil
}

// RemoveAbandonedPasswordFiles removes the password directories that a
// process ended before removing — killed in the middle of pg_dump, the
// password stays in them as plain text. A directory whose lock is held
// belongs to a running process and is kept. One without a lock file never
// received a password and is kept as well: its process may be creating it
// right now. Directories of other users and links are not touched. It
// returns how many directories were removed.
func RemoveAbandonedPasswordFiles() (int, error) {
	candidates, err := filepath.Glob(filepath.Join(os.TempDir(), passwordDirectoryPattern))
	if err != nil {
		return 0, fmt.Errorf("find abandoned PostgreSQL password directories: %w", err)
	}
	removed, failures := 0, []error(nil)
	for _, directory := range candidates {
		abandoned, err := removeIfAbandoned(directory)
		if abandoned {
			removed++
		}
		if err != nil {
			failures = append(failures, err)
		}
	}
	return removed, errors.Join(failures...)
}

func removeIfAbandoned(directory string) (bool, error) {
	own, err := ownerfile.IsOwnDirectory(directory)
	if err != nil || !own {
		return false, nil
	}
	held, err := filelock.Open(filepath.Join(directory, passwordLockName))
	if errors.Is(err, filelock.ErrLocked) || errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check PostgreSQL password directory %s: %w", directory, err)
	}
	held.Release()
	if err := os.RemoveAll(directory); err != nil {
		return false, fmt.Errorf("remove abandoned PostgreSQL password directory: %w", err)
	}
	return true, nil
}

// ToolEnvironment is the environment of a PostgreSQL client tool: the
// process environment without inherited password, ssl mode and timeout
// settings, the password file when there is one, and a bounded connection
// timeout. The ssl mode is the connection's own, added per utility run.
func ToolEnvironment(passwordFile string) []string {
	environment := make([]string, 0, len(os.Environ())+2)
	for _, variable := range os.Environ() {
		if strings.HasPrefix(variable, "PGPASSWORD=") || strings.HasPrefix(variable, "PGPASSFILE=") || strings.HasPrefix(variable, "PGCONNECT_TIMEOUT=") || strings.HasPrefix(variable, "PGSSLMODE=") {
			continue
		}
		environment = append(environment, variable)
	}
	if passwordFile != "" {
		environment = append(environment, "PGPASSFILE="+passwordFile)
	}
	return append(environment, "PGCONNECT_TIMEOUT=10")
}

// passwordLine is one line of a libpq password file. A Unix socket connection
// is matched by libpq as host "localhost".
func passwordLine(credential Credential) string {
	connection := credential.Connection
	host := connection.Host
	if strings.HasPrefix(host, "/") {
		host = "localhost"
	}
	fields := []string{host, strconv.Itoa(int(connection.Port)), connection.Database, connection.User, credential.Password}
	for index := range fields {
		fields[index] = strings.NewReplacer(`\`, `\\`, `:`, `\:`).Replace(fields[index])
	}
	return strings.Join(fields, ":") + "\n"
}
