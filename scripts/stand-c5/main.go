// Command stand-c5 raises stand С5 of the test cases of block 2в
// (docs/test-cases/blok-2v.md, «Стенды»): ML System, a developer database in
// debug mode, ML App, ML Manager and ML Studio on that database, and users
// with roles - without the window of ML Manager and without the keychain, as
// the owner decided on 22.10.2026 (question В8 of
// docs/series/2026-10-21-2.437.md).
//
// Everything runs in one process on one platform.Runtime, the way the
// integration tests of internal/platform raise it; the secrets the platform
// keeps are kept in a file of the stand's folder instead of the credential
// store of the operating system. The databases and roles of PostgreSQL are
// made with ML_TEST_ADMIN_DATABASE_URL (docs/AGENTS.md) and are named
// ml_stand_*, so that a list of the cluster tells them from those of tests.
//
//	go run ./scripts/stand-c5 up   --dir D [--project P] [--port 8190]
//	go run ./scripts/stand-c5 save --dir D [--commit C]
//	go run ./scripts/stand-c5 user --dir D --login L [--role R]...
//	go run ./scripts/stand-c5 down --dir D
//
// up makes the stand on its first run (P is the project folder, made by
// scripts/test_stand.py; nothing is saved into the database yet) and serves it
// until interrupted: the page of the stand on the port, ML App one above, ML
// Manager two above. save runs «Сохранить данные» of the project into the
// database - by this build, or by the build of commit C, which it extracts
// into the stand's folder: the cases of databases left by an earlier ML need
// that. user makes the user L, or finds the one already made, gives them ML
// App on the database and the roles R of the saved project, by name. down
// removes the databases and roles of PostgreSQL and the folder; the stand must
// not be up.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/atomicfile"
	"github.com/k33alexey/MetaLab/internal/filelock"
	"github.com/k33alexey/MetaLab/internal/gitclient"
	"github.com/k33alexey/MetaLab/internal/host"
	"github.com/k33alexey/MetaLab/internal/manager"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/portal"
	"github.com/k33alexey/MetaLab/internal/postgresadmin"
	"github.com/k33alexey/MetaLab/internal/postgresconn"
	"github.com/k33alexey/MetaLab/internal/schemadiff"
	"github.com/k33alexey/MetaLab/internal/secretstore"
	"github.com/k33alexey/MetaLab/internal/studio"
	"github.com/k33alexey/MetaLab/internal/systemdb"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

const usage = `usage:
  go run ./scripts/stand-c5 up   --dir D [--project P] [--port 8190]
  go run ./scripts/stand-c5 save --dir D [--commit C]
  go run ./scripts/stand-c5 user --dir D --login L [--role R]...
  go run ./scripts/stand-c5 down --dir D
`

// administrator is the platform administrator the stand makes; it signs in to
// ML Manager and opens ML Studio.
const administrator = "admin"

// state is what the stand remembers between runs, beside the settings of the
// platform (config.yaml) and its secrets (secrets.json). Passwords are the
// stand's own test values and are shown on the stand's page.
type state struct {
	Project   string            `json:"project"`
	Database  uuid.UUID         `json:"database"`
	Port      int               `json:"port"`
	Passwords map[string]string `json:"passwords"`
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	flags := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	dir := flags.String("dir", "", "folder of the stand")
	projectPath := flags.String("project", "", "project folder (first up)")
	port := flags.Int("port", 8190, "port of the stand's page")
	commit := flags.String("commit", "", "commit whose build saves the data")
	login := flags.String("login", "", "user's login")
	var roles roleNames
	flags.Var(&roles, "role", "role of the saved project, by name; repeats")
	_ = flags.Parse(os.Args[2:])
	if *dir == "" {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	folder, err := filepath.Abs(*dir)
	if err == nil {
		switch os.Args[1] {
		case "up":
			err = up(ctx, folder, *projectPath, *port)
		case "save":
			err = save(ctx, folder, *commit)
		case "user":
			err = addUser(ctx, folder, *login, roles)
		case "down":
			err = down(ctx, folder)
		default:
			fmt.Fprint(os.Stderr, usage)
			os.Exit(2)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "stand-c5:", err)
		os.Exit(1)
	}
}

type roleNames []string

func (names *roleNames) String() string        { return strings.Join(*names, ",") }
func (names *roleNames) Set(name string) error { *names = append(*names, name); return nil }

// up makes the stand when its folder has none and serves it until ctx ends.
// A folder that holds something other than a stand it refuses: down removes
// the whole folder, and what lay there before would go with the stand.
func up(ctx context.Context, folder, projectPath string, port int) error {
	if !holdsStand(folder) {
		entries, err := os.ReadDir(folder)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		for _, entry := range entries {
			// The lock of an up that failed before it made the stand.
			if entry.Name() != "stand.lock" {
				return fmt.Errorf("%s holds no stand and is not empty; give an empty or a new folder", folder)
			}
		}
	}
	lock, err := holdStand(folder)
	if err != nil {
		return err
	}
	defer lock.Release()
	if !holdsStand(folder) {
		if projectPath == "" {
			return fmt.Errorf("the first up needs --project")
		}
		if err := create(ctx, folder, projectPath, port); err != nil {
			return err
		}
	}
	stand, runtime, err := open(ctx, folder)
	if err != nil {
		return err
	}
	defer runtime.Close()
	settings, _, err := appconfig.Load(filepath.Join(folder, "config.yaml"))
	if err != nil {
		return err
	}
	studios := &studioWindows{root: ctx, runtime: runtime, settings: filepath.Join(folder, "studio-settings")}
	defer studios.close()
	page := fmt.Sprintf("127.0.0.1:%d", stand.Port)
	managerAddress := fmt.Sprintf("127.0.0.1:%d", stand.Port+2)
	servers := []*http.Server{
		{Addr: page, Handler: standPage(folder, stand, settings.Service.Listen, managerAddress, runtime, studios)},
		{Addr: settings.Service.Listen, Handler: portal.NewHandler(runtime)},
		{Addr: managerAddress, Handler: manager.NewHandlerWithPlatformAndStudio(settings, runtime, studios)},
	}
	failed := make(chan error, len(servers))
	for _, server := range servers {
		server.ReadHeaderTimeout = 5 * time.Second
		listener, err := net.Listen("tcp", server.Addr)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", server.Addr, err)
		}
		go func() { failed <- server.Serve(listener) }()
	}
	fmt.Printf("Стенд С5: http://%s/ (Ctrl+C — остановить)\n", page)
	select {
	case <-ctx.Done():
	case err = <-failed:
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for _, server := range servers {
		_ = server.Shutdown(shutdown)
	}
	return err
}

// holdsStand says the folder holds a stand: create writes stand.json last.
func holdsStand(folder string) bool {
	_, err := os.Stat(filepath.Join(folder, "stand.json"))
	return err == nil
}

// holdStand locks the stand's folder for the time of up: down refuses while it
// is held, and a second up does not open the same stand twice.
func holdStand(folder string) (*filelock.Lock, error) {
	if err := os.MkdirAll(folder, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(folder, "stand.lock")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		return nil, err
	}
	lock, err := filelock.Open(path)
	if errors.Is(err, filelock.ErrLocked) {
		return nil, fmt.Errorf("the stand in %s is up; stop it first", folder)
	}
	return lock, err
}

// create provisions ML System and the developer database and registers the
// latter in debug mode, as ML Manager does. What it made is taken back when a
// later step fails.
func create(ctx context.Context, folder, projectPath string, port int) error {
	projectPath, err := filepath.Abs(projectPath)
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(projectPath, "configuration.yaml")); err != nil {
		return fmt.Errorf("%s is not a project folder: %w", projectPath, err)
	}
	if err := commitProject(ctx, projectPath); err != nil {
		return err
	}
	cluster, password, err := clusterAdministrator()
	if err != nil {
		return err
	}
	settings := appconfig.Default()
	settings.SourcePath = filepath.Join(folder, "config.yaml")
	settings.Backups.Directory = filepath.Join(folder, "backups")
	settings.Service.Listen = fmt.Sprintf("127.0.0.1:%d", port+1)
	runtime := platform.New(ctx, settings, &fileSecrets{path: filepath.Join(folder, "secrets.json")})
	defer runtime.Close()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	if _, err := runtime.ProvisionPostgreSQL(ctx, platform.ProvisionRequest{
		Host: cluster.Host, Port: cluster.Port, AdministratorDatabase: cluster.Database,
		AdministratorUser: cluster.User, AdministratorPassword: password, SSLMode: cluster.SSLMode,
		SystemDatabase: "ml_stand_sys_" + suffix, TechnicalUser: "ml_stand_sys_u_" + suffix,
	}); err != nil {
		return err
	}
	made := []postgresadmin.Provisioned{{Connection: *runtime.State().Connection}}
	undo := func(cause error) error {
		runtime.Close()
		for index := len(made) - 1; index >= 0; index-- {
			cause = errors.Join(cause, postgresadmin.RollbackProvisioned(context.WithoutCancel(ctx), cluster, password, made[index]))
		}
		return errors.Join(cause, os.Remove(settings.SourcePath), os.Remove(filepath.Join(folder, "secrets.json")))
	}
	stand := state{Project: projectPath, Port: port, Passwords: map[string]string{administrator: standPassword(administrator)}}
	if _, _, err := runtime.CreateInitial(ctx, administrator, stand.Passwords[administrator]); err != nil {
		return undo(err)
	}
	application, err := postgresadmin.Provision(ctx, cluster, password, "ml_stand_app_"+suffix, "ml_stand_app_u_"+suffix)
	if err != nil {
		return undo(err)
	}
	made = append(made, application)
	registered, err := runtime.RegisterDatabase(ctx, platform.RegisterDatabaseRequest{
		Name: "Стенд С5", Host: application.Connection.Host, Port: application.Connection.Port,
		Database: application.Connection.Database, User: application.Connection.User,
		Password: application.Password, SSLMode: application.Connection.SSLMode, Mode: systemdb.DatabaseDebug,
	})
	if err != nil {
		return undo(err)
	}
	if _, err := runtime.StartDatabase(ctx, registered.ID); err != nil {
		return undo(err)
	}
	stand.Database = registered.ID
	if err := writeJSON(filepath.Join(folder, "stand.json"), stand); err != nil {
		return undo(err)
	}
	return nil
}

// commitProject makes the project a repository of Git with everything
// committed, when it is not one: «Сохранить данные» reads the project through
// Git, and the folders scripts/test_stand.py makes are plain copies.
func commitProject(ctx context.Context, projectPath string) error {
	if _, err := os.Stat(filepath.Join(projectPath, ".git")); err == nil {
		return nil
	}
	for _, arguments := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.name", "Стенд С5"},
		{"config", "user.email", "stand-c5@example.invalid"},
		{"add", "--all"},
		{"commit", "-q", "-m", "Проект стенда С5"},
	} {
		if output, err := exec.CommandContext(ctx, "git", append([]string{"-C", projectPath}, arguments...)...).CombinedOutput(); err != nil {
			return fmt.Errorf("git %s: %w\n%s", arguments[0], err, output)
		}
	}
	return nil
}

// standPassword is the test password of a login on the stand.
func standPassword(login string) string { return login + "-stand-c5" }

// clusterAdministrator reads ML_TEST_ADMIN_DATABASE_URL, the superuser of the
// test cluster that provisions throwaway databases (docs/AGENTS.md).
func clusterAdministrator() (postgresconn.Descriptor, string, error) {
	address := os.Getenv("ML_TEST_ADMIN_DATABASE_URL")
	if address == "" {
		return postgresconn.Descriptor{}, "", fmt.Errorf("ML_TEST_ADMIN_DATABASE_URL is not set (docs/AGENTS.md)")
	}
	settings, err := pgxpool.ParseConfig(address)
	if err != nil {
		return postgresconn.Descriptor{}, "", err
	}
	parsed, err := url.Parse(address)
	if err != nil {
		return postgresconn.Descriptor{}, "", err
	}
	sslMode := parsed.Query().Get("sslmode")
	if sslMode == "" {
		sslMode = "require"
	}
	return postgresconn.Descriptor{
		Host: settings.ConnConfig.Host, Port: settings.ConnConfig.Port,
		Database: settings.ConnConfig.Database, User: settings.ConnConfig.User,
		SSLMode: sslMode, SecretKey: "stand.cluster.password",
	}, settings.ConnConfig.Password, nil
}

// open reads the stand and opens the platform on its settings and secrets.
func open(ctx context.Context, folder string) (state, *platform.Runtime, error) {
	var stand state
	content, err := os.ReadFile(filepath.Join(folder, "stand.json"))
	if err != nil {
		return stand, nil, fmt.Errorf("no stand in %s: %w", folder, err)
	}
	if err := json.Unmarshal(content, &stand); err != nil {
		return stand, nil, err
	}
	settings, _, err := appconfig.Load(filepath.Join(folder, "config.yaml"))
	if err != nil {
		return stand, nil, err
	}
	runtime := platform.New(ctx, settings, &fileSecrets{path: filepath.Join(folder, "secrets.json")})
	if state := runtime.State(); !state.Connected {
		runtime.Close()
		return stand, nil, fmt.Errorf("ML System is not reachable: %s", state.Error)
	}
	return stand, runtime, nil
}

// managerContext signs the stand's administrator in to ML Manager.
func managerContext(ctx context.Context, runtime *platform.Runtime, stand state) (context.Context, error) {
	login, err := runtime.LoginManager(ctx, administrator, stand.Passwords[administrator], "127.0.0.1", "stand-c5")
	if err != nil {
		return nil, fmt.Errorf("sign in to ML Manager: %w", err)
	}
	return platform.WithManagerToken(ctx, login.Token), nil
}

// save runs «Сохранить данные» of the stand's project into its database.
func save(ctx context.Context, folder, commit string) error {
	stand, runtime, err := open(ctx, folder)
	if err != nil {
		return err
	}
	defer runtime.Close()
	if commit == "" {
		saved, _, err := runtime.SaveApplicationData(ctx, stand.Database, stand.Project, schemadiff.MigrationConsent{})
		if err != nil {
			return err
		}
		fmt.Printf("Сохранено текущей сборкой: %+v\n", saved)
		return nil
	}
	databaseURL, err := applicationURL(ctx, runtime, stand.Database)
	if err != nil {
		return err
	}
	build, err := extractBuild(ctx, folder, commit)
	if err != nil {
		return err
	}
	command := exec.CommandContext(ctx, "go", "run", "./stand-c5-save", stand.Project)
	command.Dir = build
	command.Env = append(os.Environ(), "ML_STAND_DATABASE_URL="+databaseURL, "ML_STAND_COMMIT="+commit)
	command.Stdout, command.Stderr = os.Stdout, os.Stderr
	if err := command.Run(); err != nil {
		return fmt.Errorf("save by the build of %s: %w", commit, err)
	}
	return nil
}

// applicationURL is the address of the developer database with its password,
// for a build that has no stand of its own.
func applicationURL(ctx context.Context, runtime *platform.Runtime, databaseID uuid.UUID) (string, error) {
	pool, registered, err := runtime.OpenDebugDatabase(ctx, databaseID)
	if err != nil {
		return "", err
	}
	password := pool.Config().ConnConfig.Password
	pool.Close()
	connection := registered.Connection
	address := url.URL{
		Scheme: "postgres", User: url.UserPassword(connection.User, password),
		Host: net.JoinHostPort(connection.Host, strconv.Itoa(int(connection.Port))), Path: "/" + connection.Database,
		RawQuery: "sslmode=" + url.QueryEscape(connection.SSLMode),
	}
	return address.String(), nil
}

// savingProgram runs «Сохранить данные» the way every build since 2.332 does
// it: publication.SaveData in debug mode, confirmed, with no consent to lose
// data. It is written into the extracted build and compiled there, against
// that build's own publication.
const savingProgram = `package main

import (
	"context"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/k33alexey/MetaLab/internal/publication"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("ML_STAND_DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	saved, _, err := publication.SaveData(ctx, pool, publication.SaveDataRequest{Root: os.Args[1], Mode: publication.ActivationDebug, Confirmed: true})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("Сохранено сборкой %s: %+v\n", os.Getenv("ML_STAND_COMMIT"), saved)
}
`

// extractBuild puts the sources of commit into builds/<commit> of the stand,
// with the saving program beside them; the repository is not touched.
func extractBuild(ctx context.Context, folder, commit string) (string, error) {
	repository, err := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", fmt.Errorf("run from the repository: %w", err)
	}
	full, err := exec.CommandContext(ctx, "git", "rev-parse", "--verify", commit+"^{commit}").Output()
	if err != nil {
		return "", fmt.Errorf("no commit %s", commit)
	}
	build := filepath.Join(folder, "builds", strings.TrimSpace(string(full)))
	if _, err := os.Stat(filepath.Join(build, "go.mod")); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(build, 0o700); err != nil {
			return "", err
		}
		archive := exec.CommandContext(ctx, "git", "-C", strings.TrimSpace(string(repository)), "archive", "--format=tar", strings.TrimSpace(string(full)))
		unpack := exec.CommandContext(ctx, "tar", "-x", "-C", build)
		unpack.Stdin, err = archive.StdoutPipe()
		if err != nil {
			return "", err
		}
		if err := unpack.Start(); err != nil {
			return "", err
		}
		if err := errors.Join(archive.Run(), unpack.Wait()); err != nil {
			return "", fmt.Errorf("extract %s: %w", commit, err)
		}
	}
	program := filepath.Join(build, "stand-c5-save", "main.go")
	if err := os.MkdirAll(filepath.Dir(program), 0o700); err != nil {
		return "", err
	}
	return build, os.WriteFile(program, []byte(savingProgram), 0o600)
}

// addUser makes the user, or finds them, and gives them ML App on the
// stand's database with the named roles of the saved project, as the
// administrator does in ML Manager. A new user's password is changed once
// on their behalf, so that ML App does not ask for it on the first sign-in.
func addUser(ctx context.Context, folder, login string, roles []string) error {
	if login == "" {
		return fmt.Errorf("user needs --login")
	}
	stand, runtime, err := open(ctx, folder)
	if err != nil {
		return err
	}
	defer runtime.Close()
	managerCtx, err := managerContext(ctx, runtime, stand)
	if err != nil {
		return err
	}
	userID, err := ensureUser(ctx, managerCtx, runtime, &stand, login)
	if err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(folder, "stand.json"), stand); err != nil {
		return err
	}
	if err := runtime.SetManagerDatabasePermissions(managerCtx, userID, stand.Database, systemdb.DatabasePermissions{App: true}); err != nil {
		return err
	}
	view, err := runtime.GetManagerApplicationRoles(managerCtx, stand.Database, userID)
	if err != nil {
		return fmt.Errorf("roles of the saved project: %w", err)
	}
	update := platform.ApplicationRoleUpdate{ProjectID: view.ProjectID, ExpectedRevision: view.Assignment.Revision}
	for _, name := range roles {
		found := false
		for _, option := range view.Available {
			if option.Name == name {
				update.RoleIDs, found = append(update.RoleIDs, option.ID), true
			}
		}
		if !found {
			return fmt.Errorf("the saved project has no role %q", name)
		}
	}
	if _, err := runtime.SetManagerApplicationRoles(managerCtx, stand.Database, userID, update); err != nil {
		return err
	}
	fmt.Printf("Пользователь %s: ML App, роли %v\n", login, roles)
	return nil
}

func ensureUser(ctx, managerCtx context.Context, runtime *platform.Runtime, stand *state, login string) (uuid.UUID, error) {
	users, err := runtime.ListManagerUsers(managerCtx)
	if err != nil {
		return uuid.UUID{}, err
	}
	for _, user := range users {
		if user.Login == login {
			return user.ID, nil
		}
	}
	first := standPassword(login) + "-first"
	user, err := runtime.CreateManagerUser(managerCtx, systemdb.UserCreation{Login: login, Password: first})
	if err != nil {
		return uuid.UUID{}, err
	}
	session, err := runtime.LoginPortal(ctx, login, first, "127.0.0.1", "stand-c5")
	if err != nil {
		return uuid.UUID{}, err
	}
	if err := runtime.ChangePortalPassword(ctx, session.Token, first, standPassword(login)); err != nil {
		return uuid.UUID{}, err
	}
	_ = runtime.LogoutPortal(ctx, session.Token)
	stand.Passwords[login] = standPassword(login)
	return user.ID, nil
}

// down removes what create made: the databases and roles of PostgreSQL, then
// the stand's folder.
func down(ctx context.Context, folder string) error {
	// Nothing is made or removed in a folder with no stand: holdStand would
	// make the folder and its lock first.
	if !holdsStand(folder) {
		return fmt.Errorf("%s holds no stand", folder)
	}
	lock, err := holdStand(folder)
	if err != nil {
		return err
	}
	stand, runtime, err := open(ctx, folder)
	if err != nil {
		lock.Release()
		return err
	}
	cluster, password, err := clusterAdministrator()
	if err != nil {
		runtime.Close()
		lock.Release()
		return err
	}
	made := []postgresadmin.Provisioned{{Connection: *runtime.State().Connection}}
	if databases, err := runtime.ListDatabases(ctx); err == nil {
		for _, database := range databases {
			if database.ID == stand.Database {
				made = append(made, postgresadmin.Provisioned{Connection: database.Connection})
			}
		}
	}
	runtime.Close()
	for index := len(made) - 1; index >= 0; index-- {
		err = errors.Join(err, postgresadmin.RollbackProvisioned(ctx, cluster, password, made[index]))
	}
	lock.Release()
	if err != nil {
		return err
	}
	return os.RemoveAll(folder)
}

func writeJSON(path string, value any) error {
	return atomicfile.Write(path, ".stand-*.json", 0o600, func(writer io.Writer) error {
		encoder := json.NewEncoder(writer)
		encoder.SetIndent("", "  ")
		return encoder.Encode(value)
	})
}

// fileSecrets is platform.Secrets in one file of the stand: the stand's
// databases are throwaway, and the keychain is not touched.
type fileSecrets struct {
	mu   sync.Mutex
	path string
}

func (store *fileSecrets) read() (map[string]string, error) {
	values := map[string]string{}
	content, err := os.ReadFile(store.path)
	if errors.Is(err, os.ErrNotExist) {
		return values, nil
	}
	if err != nil {
		return nil, err
	}
	return values, json.Unmarshal(content, &values)
}

func (store *fileSecrets) change(key string, change func(map[string]string)) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	values, err := store.read()
	if err != nil {
		return err
	}
	change(values)
	return writeJSON(store.path, values)
}

func (store *fileSecrets) Set(key, value string) error {
	return store.change(key, func(values map[string]string) { values[key] = value })
}

func (store *fileSecrets) Delete(key string) error {
	return store.change(key, func(values map[string]string) { delete(values, key) })
}

func (store *fileSecrets) Get(key string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	values, err := store.read()
	if err != nil {
		return "", err
	}
	value, exists := values[key]
	if !exists {
		return "", secretstore.ErrNotFound
	}
	return value, nil
}

// studioWindows opens ML Studio on the stand the way the window of ML Studio
// is opened from ML Manager: a lease taken by the signed-in administrator,
// then host.ServeStudio. A browser tab stands for the window, and opening
// Studio again closes the one before, as only one Studio holds a database.
type studioWindows struct {
	mu       sync.Mutex
	root     context.Context
	runtime  *platform.Runtime
	settings string
	// shut closes the Studio open now, if one is.
	shut func()
}

// OpenStudio is what the button of ML Manager calls; the address of Studio
// goes to the stand's output and page.
func (windows *studioWindows) OpenStudio(ctx context.Context, databaseID uuid.UUID, projectPath string) error {
	address, err := windows.open(ctx, databaseID, projectPath)
	if err == nil {
		fmt.Println("ML Studio:", address)
	}
	return err
}

func (windows *studioWindows) CloneProject(ctx context.Context, repository, destination string) error {
	return gitclient.Clone(ctx, repository, destination)
}

func (windows *studioWindows) open(ctx context.Context, databaseID uuid.UUID, projectPath string) (string, error) {
	windows.mu.Lock()
	defer windows.mu.Unlock()
	windows.closeLocked()
	workspace, err := studio.OpenForDatabase(projectPath, databaseID)
	if err != nil {
		return "", fmt.Errorf("open ML Project: %w", err)
	}
	workspace.KeepUserSettingsIn(windows.settings)
	snapshot, err := workspace.Snapshot()
	if err != nil {
		return "", fmt.Errorf("read ML Project: %w", err)
	}
	lease, err := windows.runtime.AcquireStudioSession(ctx, databaseID, snapshot.Configuration.ID, "", "stand-c5", int64(os.Getpid()))
	if err != nil {
		return "", err
	}
	served, err := host.ServeStudio(windows.runtime, workspace, databaseID, lease)
	if err != nil {
		host.ReleaseStudioLease(ctx, windows.runtime, lease)
		return "", err
	}
	heartbeat, stopHeartbeat := context.WithCancel(windows.root)
	shut := func() {
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = served.Shutdown(shutdown)
	}
	go func() {
		if err := host.KeepStudioLease(heartbeat, windows.runtime, lease, shut); err != nil {
			slog.Warn("ML Studio of the stand closed", "error", err)
		}
	}()
	windows.shut = func() {
		stopHeartbeat()
		shut()
		host.ReleaseStudioLease(windows.root, windows.runtime, lease)
	}
	return served.URL, nil
}

func (windows *studioWindows) closeLocked() {
	if windows.shut != nil {
		windows.shut()
		windows.shut = nil
	}
}

func (windows *studioWindows) close() {
	windows.mu.Lock()
	defer windows.mu.Unlock()
	windows.closeLocked()
}

// standPage is the stand's own page: where ML App and ML Manager are, the test
// accounts, and «Открыть ML Studio», which signs the administrator in and
// sends the browser to a fresh Studio - its launch address is good for one
// open within a minute.
func standPage(folder string, stand state, portalAddress, managerAddress string, runtime *platform.Runtime, studios *studioWindows) http.Handler {
	routes := http.NewServeMux()
	routes.HandleFunc("GET /{$}", func(response http.ResponseWriter, request *http.Request) {
		// user adds accounts while the stand is up. Each request reads them
		// into its own copy: the stand of up is shared by every request, and
		// two at once would write and read one map (review 2.464, П3).
		stand := stand
		if current, err := os.ReadFile(filepath.Join(folder, "stand.json")); err == nil {
			var fresh state
			if json.Unmarshal(current, &fresh) == nil {
				stand = fresh
			}
		}
		response.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(response, `<!doctype html><html lang="ru"><meta charset="utf-8"><title>Стенд С5</title>
<body style="font:15px/1.5 system-ui;margin:24px">
<h1>Стенд С5</h1>
<p>Проект: <code>%s</code><br>База разработчика: <code>%s</code> (режим «Отладка»)</p>
<ul>
<li><a href="/studio">Открыть ML Studio</a></li>
<li><a href="http://%s/">ML App (портал)</a></li>
<li><a href="http://%s/">ML Manager</a></li>
</ul>
<h2>Учётные записи (тестовые)</h2><table>`, html.EscapeString(stand.Project), stand.Database, portalAddress, managerAddress)
		for login, password := range stand.Passwords {
			fmt.Fprintf(response, "<tr><td><code>%s</code></td><td><code>%s</code></td></tr>", html.EscapeString(login), html.EscapeString(password))
		}
		fmt.Fprint(response, "</table></body></html>")
	})
	routes.HandleFunc("GET /studio", func(response http.ResponseWriter, request *http.Request) {
		managerCtx, err := managerContext(request.Context(), runtime, stand)
		if err == nil {
			var address string
			address, err = studios.open(managerCtx, stand.Database, stand.Project)
			if err == nil {
				http.Redirect(response, request, address, http.StatusSeeOther)
				return
			}
		}
		http.Error(response, err.Error(), http.StatusBadGateway)
	})
	return routes
}
