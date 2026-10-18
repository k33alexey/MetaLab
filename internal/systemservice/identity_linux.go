//go:build linux

package systemservice

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"

	kservice "github.com/kardianos/service"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/servicestate"
)

// ML Service on Linux is a system service under its own account: the secrets
// belong to that account (PLATFORM-MODEL.md, «Секреты в Linux»), and a user
// service does not start at boot and stops when its user logs out.
func configureServiceIdentity(configuration *kservice.Config) {
	configuration.UserName = servicestate.Account
}

// systemdPlatform is what kardianos/service reports under systemd; the
// account a service runs under is written only into a systemd unit.
const systemdPlatform = "linux-systemd"

// nologinShells are where distributions keep the shell that refuses a login.
var nologinShells = []string{"/usr/sbin/nologin", "/sbin/nologin", "/bin/false"}

func prepareInstall(platform, settingsPath string) error {
	return systemInstall().prepare(platform, settingsPath)
}

// installHost is what preparing an install asks of the machine; tests
// replace it, since the real one needs root.
type installHost struct {
	euid       func() int
	lookup     func(string) (*user.User, error)
	addAccount func() error
	claimState func() error
	// permits reports whether the account may read path, or create in it
	// when write is set.
	permits func(account *user.User, path string, write bool) error
}

func systemInstall() installHost {
	return installHost{
		euid: os.Geteuid, lookup: user.Lookup, addAccount: addServiceAccount,
		claimState: servicestate.Claim, permits: accountPermits,
	}
}

// prepare makes the machine ready for the service before its unit is
// written: the account exists, owns its state directory with the folders of
// the secrets and the backups, and reaches the settings and a configured
// backup folder. Rights on the administrator's own files are not changed —
// install names the path the account cannot reach.
func (host installHost) prepare(platform, settingsPath string) error {
	if platform != systemdPlatform {
		return fmt.Errorf("ML Service on Linux is installed under systemd only, this system uses %q", platform)
	}
	if host.euid() != 0 {
		return fmt.Errorf("installing ML Service on Linux requires root: it creates the service account %q and a system service", servicestate.Account)
	}
	settings, _, err := appconfig.Load(settingsPath)
	if err != nil {
		return err
	}
	account, err := host.lookup(servicestate.Account)
	var unknown user.UnknownUserError
	if errors.As(err, &unknown) {
		if err := host.addAccount(); err != nil {
			return err
		}
		account, err = host.lookup(servicestate.Account)
	}
	if err != nil {
		return fmt.Errorf("service account %q is not available: %w", servicestate.Account, err)
	}
	if err := host.claimState(); err != nil {
		return err
	}
	if err := host.permits(account, settingsPath, false); err != nil {
		return fmt.Errorf("service account %q cannot read settings %s: %w", servicestate.Account, settingsPath, err)
	}
	// Without a folder in the settings the service keeps backups in its own
	// folder, given to it with the state directory.
	if settings.Backups.Directory == "" {
		return nil
	}
	folder, err := existingFolder(settings.Backups.Directory)
	if err != nil {
		return err
	}
	if err := host.permits(account, folder, true); err != nil {
		return fmt.Errorf("service account %q cannot write backups to %s: %w", servicestate.Account, folder, err)
	}
	return nil
}

// existingFolder is the backup folder, or the nearest folder above it that
// exists: the service creates the rest when it writes the first archive.
func existingFolder(path string) (string, error) {
	for {
		information, err := os.Stat(path)
		if err == nil {
			if !information.IsDir() {
				return "", fmt.Errorf("backup directory %s is not a directory", path)
			}
			return path, nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("read backup directory %s: %w", path, err)
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", fmt.Errorf("backup directory %s has no existing folder above it", path)
		}
		path = parent
	}
}

// addServiceAccount creates the system account without a login, its home
// being the state directory.
func addServiceAccount() error {
	useradd, err := exec.LookPath("useradd")
	if err != nil {
		return fmt.Errorf("create service account %q: useradd is not available, create a system account without a login: %w", servicestate.Account, err)
	}
	shell := nologinShells[len(nologinShells)-1]
	for _, candidate := range nologinShells {
		if _, err := os.Stat(candidate); err == nil {
			shell = candidate
			break
		}
	}
	output, err := exec.Command(useradd, "--system", "--user-group", "--no-create-home",
		"--home-dir", servicestate.StateDirectory, "--shell", shell,
		"--comment", "MetaLab Service", servicestate.Account).CombinedOutput()
	if err != nil {
		return fmt.Errorf("create service account %q: %w: %s", servicestate.Account, err, output)
	}
	return nil
}

// accountPermits asks the system itself, by running test(1) under the
// account with its groups: rights, access lists and every folder on the way
// are then judged as they will be for the service.
func accountPermits(account *user.User, path string, write bool) error {
	uid, gid, err := servicestate.AccountIDs(account)
	if err != nil {
		return err
	}
	groupIDs, err := account.GroupIds()
	if err != nil {
		return fmt.Errorf("read groups of %q: %w", account.Username, err)
	}
	groups := make([]uint32, 0, len(groupIDs))
	for _, id := range groupIDs {
		group, err := strconv.ParseUint(id, 10, 32)
		if err != nil {
			return fmt.Errorf("account %q has group id %q: %w", account.Username, id, err)
		}
		groups = append(groups, uint32(group))
	}
	test, err := exec.LookPath("test")
	if err != nil {
		return fmt.Errorf("check access: %w", err)
	}
	checks := [][]string{{"-r", path}}
	if write {
		checks = [][]string{{"-w", path}, {"-x", path}}
	}
	for _, check := range checks {
		command := exec.Command(test, check...)
		command.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{
			Uid: uint32(uid), Gid: uint32(gid), Groups: groups,
		}}
		if err := command.Run(); err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return errors.New("permission denied")
			}
			return fmt.Errorf("check access: %w", err)
		}
	}
	return nil
}
