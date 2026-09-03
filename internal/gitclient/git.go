// Package gitclient provides a bounded, non-interactive Git surface for ML Studio.
package gitclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/k33alexey/MetaLab/internal/project"
)

const maxGitOutput = 2 << 20

var (
	ErrGitUnavailable   = errors.New("Git is not installed or unavailable")
	ErrNotRepository    = errors.New("ML Project is not a Git repository")
	ErrUnsafeRepository = errors.New("Git repository root must match the ML Project directory")
	ErrDirtyWorktree    = errors.New("Git working tree contains uncommitted changes")
)

var credentialURL = regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://)[^/@[:space:]]+@`)

type Status struct {
	Branch   string        `json:"branch"`
	Revision string        `json:"revision,omitempty"`
	Ahead    int           `json:"ahead"`
	Behind   int           `json:"behind"`
	Entries  []StatusEntry `json:"entries"`
}

type StatusEntry struct {
	Path         string `json:"path"`
	OriginalPath string `json:"originalPath,omitempty"`
	Index        string `json:"index"`
	Worktree     string `json:"worktree"`
	Conflicted   bool   `json:"conflicted"`
}

type Branches struct {
	Current string   `json:"current"`
	Items   []string `json:"items"`
}

type Result struct {
	Output string `json:"output"`
}

type Client struct{ root string }

func Open(ctx context.Context, root string) (*Client, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve ML Project path: %w", err)
	}
	client := &Client{root: filepath.Clean(absolute)}
	repositoryRoot, err := client.run(ctx, "rev-parse", "--show-toplevel")
	if err != nil {
		if errors.Is(err, ErrGitUnavailable) {
			return nil, err
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		return nil, ErrNotRepository
	}
	repositoryRoot = strings.TrimSpace(repositoryRoot)
	projectInfo, projectErr := os.Stat(client.root)
	repositoryInfo, repositoryErr := os.Stat(repositoryRoot)
	if projectErr != nil || repositoryErr != nil || !os.SameFile(projectInfo, repositoryInfo) {
		return nil, ErrUnsafeRepository
	}
	return client, nil
}

func Clone(ctx context.Context, remote, destination string) error {
	remote = strings.TrimSpace(remote)
	if remote == "" {
		return fmt.Errorf("Git repository address is required")
	}
	if parsed, err := url.Parse(remote); err == nil && parsed.IsAbs() && parsed.User != nil {
		_, hasPassword := parsed.User.Password()
		if hasPassword || parsed.Scheme == "http" || parsed.Scheme == "https" {
			return fmt.Errorf("Git repository address must not contain credentials")
		}
	}
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return fmt.Errorf("clone destination is required")
	}
	absolute, err := filepath.Abs(destination)
	if err != nil {
		return fmt.Errorf("resolve clone destination: %w", err)
	}
	if _, err := os.Stat(absolute); err == nil {
		return fmt.Errorf("clone destination already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect clone destination: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return fmt.Errorf("create clone parent directory: %w", err)
	}
	if _, err := runAt(ctx, filepath.Dir(absolute), "clone", "--", remote, absolute); err != nil {
		return err
	}
	if _, err := project.ValidateLayout(absolute); err != nil {
		return fmt.Errorf("cloned repository is not an ML Project: %w", err)
	}
	return nil
}

func (client *Client) Status(ctx context.Context) (Status, error) {
	output, err := client.run(ctx, "status", "--porcelain=v1", "-z", "--branch", "--untracked-files=all")
	if err != nil {
		return Status{}, err
	}
	status, err := parseStatus(output)
	if err != nil {
		return Status{}, err
	}
	if revision, revisionErr := client.run(ctx, "rev-parse", "--verify", "HEAD"); revisionErr == nil {
		status.Revision = strings.TrimSpace(revision)
	}
	return status, nil
}

func (client *Client) Diff(ctx context.Context, relativePath string) (string, error) {
	arguments := []string{"diff", "--no-ext-diff", "--no-color"}
	if relativePath != "" {
		path, err := client.relativePath(relativePath)
		if err != nil {
			return "", err
		}
		arguments = append(arguments, "--", path)
	}
	unstaged, err := client.run(ctx, arguments...)
	if err != nil {
		return "", err
	}
	stagedArguments := []string{"diff", "--cached", "--no-ext-diff", "--no-color"}
	if relativePath != "" {
		stagedArguments = append(stagedArguments, "--", filepath.ToSlash(filepath.Clean(relativePath)))
	}
	staged, err := client.run(ctx, stagedArguments...)
	if err != nil {
		return "", err
	}
	var result strings.Builder
	if staged != "" {
		result.WriteString("# Staged changes\n")
		result.WriteString(staged)
	}
	if unstaged != "" {
		if result.Len() > 0 {
			result.WriteByte('\n')
		}
		result.WriteString("# Working tree changes\n")
		result.WriteString(unstaged)
	}
	return result.String(), nil
}

func (client *Client) Commit(ctx context.Context, message string) (Result, error) {
	message = strings.TrimSpace(message)
	if message == "" || len(message) > 4096 {
		return Result{}, fmt.Errorf("Git commit message must contain 1 to 4096 bytes")
	}
	if _, err := client.run(ctx, "add", "--all", "--", "."); err != nil {
		return Result{}, err
	}
	output, err := client.run(ctx, "commit", "-m", message)
	return Result{Output: output}, err
}

func (client *Client) Pull(ctx context.Context) (Result, error) {
	status, err := client.Status(ctx)
	if err != nil {
		return Result{}, err
	}
	if len(status.Entries) != 0 {
		return Result{}, ErrDirtyWorktree
	}
	output, err := client.run(ctx, "pull", "--ff-only")
	return Result{Output: output}, err
}

func (client *Client) Push(ctx context.Context) (Result, error) {
	output, err := client.run(ctx, "push")
	return Result{Output: output}, err
}

func (client *Client) Branches(ctx context.Context) (Branches, error) {
	output, err := client.run(ctx, "for-each-ref", "--format=%(refname:short)%00", "refs/heads")
	if err != nil {
		return Branches{}, err
	}
	current, err := client.run(ctx, "branch", "--show-current")
	if err != nil {
		return Branches{}, err
	}
	items := make([]string, 0)
	for _, item := range strings.Split(output, "\x00") {
		if item = strings.TrimSpace(item); item != "" {
			items = append(items, item)
		}
	}
	return Branches{Current: strings.TrimSpace(current), Items: items}, nil
}

func (client *Client) SwitchBranch(ctx context.Context, name string, create bool) (Result, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Result{}, fmt.Errorf("Git branch name is required")
	}
	if _, err := client.run(ctx, "check-ref-format", "--branch", name); err != nil {
		return Result{}, fmt.Errorf("invalid Git branch name")
	}
	status, err := client.Status(ctx)
	if err != nil {
		return Result{}, err
	}
	if len(status.Entries) != 0 {
		return Result{}, ErrDirtyWorktree
	}
	arguments := []string{"switch"}
	if create {
		arguments = append(arguments, "-c")
	}
	output, err := client.run(ctx, append(arguments, name)...)
	return Result{Output: output}, err
}

func (client *Client) relativePath(value string) (string, error) {
	if value == "" || filepath.IsAbs(value) || strings.Contains(value, "\\") {
		return "", fmt.Errorf("invalid Git path")
	}
	clean := filepath.Clean(filepath.FromSlash(value))
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid Git path")
	}
	return filepath.ToSlash(clean), nil
}

func (client *Client) run(ctx context.Context, arguments ...string) (string, error) {
	return runAt(ctx, client.root, arguments...)
}

func runAt(ctx context.Context, directory string, arguments ...string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", ErrGitUnavailable
	}
	commandContext, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	command := exec.CommandContext(commandContext, "git", append([]string{"-C", directory}, arguments...)...)
	command.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "GIT_MERGE_AUTOEDIT=no", "GCM_INTERACTIVE=Never", "GIT_SSH_COMMAND=ssh -oBatchMode=yes")
	output := &limitedBuffer{limit: maxGitOutput}
	command.Stdout, command.Stderr = output, output
	err := command.Run()
	text := redactCredentials(strings.TrimSpace(output.String()))
	if err != nil {
		if commandContext.Err() != nil {
			return "", commandContext.Err()
		}
		if text == "" {
			text = err.Error()
		}
		return "", fmt.Errorf("Git command failed: %s", text)
	}
	return text, nil
}

func parseStatus(output string) (Status, error) {
	fields := strings.Split(output, "\x00")
	status := Status{Entries: make([]StatusEntry, 0)}
	for index := 0; index < len(fields); index++ {
		field := fields[index]
		if field == "" {
			continue
		}
		if strings.HasPrefix(field, "## ") {
			header := strings.TrimPrefix(field, "## ")
			header = strings.TrimPrefix(header, "No commits yet on ")
			branch, tracking, _ := strings.Cut(header, "...")
			status.Branch = strings.TrimSpace(branch)
			if position := strings.Index(tracking, " ["); position >= 0 {
				counts := strings.TrimSuffix(tracking[position+2:], "]")
				_, _ = fmt.Sscanf(counts, "ahead %d, behind %d", &status.Ahead, &status.Behind)
				if status.Ahead == 0 {
					_, _ = fmt.Sscanf(counts, "ahead %d", &status.Ahead)
				}
				if status.Behind == 0 {
					_, _ = fmt.Sscanf(counts, "behind %d", &status.Behind)
				}
			}
			continue
		}
		if len(field) < 4 || field[2] != ' ' {
			return Status{}, fmt.Errorf("parse Git status entry")
		}
		entry := StatusEntry{Index: field[:1], Worktree: field[1:2], Path: field[3:]}
		entry.Conflicted = strings.Contains("DD AU UD UA DU AA UU", field[:2])
		if (entry.Index == "R" || entry.Index == "C") && index+1 < len(fields) {
			index++
			entry.OriginalPath = fields[index]
		}
		status.Entries = append(status.Entries, entry)
	}
	if status.Branch == "HEAD (no branch)" {
		status.Branch = "(detached)"
	}
	return status, nil
}

func redactCredentials(value string) string { return credentialURL.ReplaceAllString(value, `${1}***@`) }

type limitedBuffer struct {
	buffer bytes.Buffer
	limit  int
}

func (buffer *limitedBuffer) Write(value []byte) (int, error) {
	originalLength := len(value)
	remaining := buffer.limit - buffer.buffer.Len()
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		_, _ = buffer.buffer.Write(value)
	}
	return originalLength, nil
}

func (buffer *limitedBuffer) String() string { return buffer.buffer.String() }
