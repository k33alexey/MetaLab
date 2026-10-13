package postgresconn

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/k33alexey/MetaLab/internal/ownerfile"
)

// Credential is a connection with the password a PostgreSQL client tool uses for it.
type Credential struct {
	Connection Descriptor
	Password   string
}

// WithPasswordFile hands passwords to PostgreSQL client tools (pg_dump,
// pg_restore) through a password file, never through arguments or
// PGPASSWORD, which other users of the machine may see. The file lies in a
// new owner-only temporary directory, not beside the data the tool works on,
// and is created closed before the passwords are written. The directory is
// removed when action returns; action receives the tool environment.
func WithPasswordFile(credentials []Credential, action func(environment []string) error) error {
	directory, err := ownerfile.MkdirTemp("", "metalab-pgpass-*")
	if err != nil {
		return fmt.Errorf("create PostgreSQL password directory: %w", err)
	}
	defer os.RemoveAll(directory)
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
