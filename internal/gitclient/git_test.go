package gitclient

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k33alexey/MetaLab/internal/project"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

func TestClientStatusDiffCommitAndBranches(t *testing.T) {
	requireGit(t)
	root := initializedRepository(t)
	client, err := Open(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	modulePath, err := project.ModulePath(uuid.MustNew())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(modulePath)), []byte("Процедура Тест()\nКонецПроцедуры\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	status, err := client.Status(context.Background())
	if err != nil || status.Branch != "main" || status.Revision == "" || len(status.Entries) != 1 || status.Entries[0].Path != modulePath {
		t.Fatalf("status=%+v error=%v", status, err)
	}
	if _, err := client.Diff(context.Background(), "../outside"); err == nil {
		t.Fatal("Diff accepted path traversal")
	}
	result, err := client.Commit(context.Background(), "Add test module")
	if err != nil || !strings.Contains(result.Output, "Add test module") {
		t.Fatalf("commit=%+v error=%v", result, err)
	}
	status, err = client.Status(context.Background())
	if err != nil || len(status.Entries) != 0 {
		t.Fatalf("clean status=%+v error=%v", status, err)
	}
	if _, err := client.SwitchBranch(context.Background(), "feature/catalogs", true); err != nil {
		t.Fatal(err)
	}
	branches, err := client.Branches(context.Background())
	if err != nil || branches.Current != "feature/catalogs" || !contains(branches.Items, "main") || !contains(branches.Items, "feature/catalogs") {
		t.Fatalf("branches=%+v error=%v", branches, err)
	}
}

func TestClonePushAndFastForwardPull(t *testing.T) {
	requireGit(t)
	ctx := context.Background()
	source := initializedRepository(t)
	bare := filepath.Join(t.TempDir(), "remote.git")
	runGit(t, t.TempDir(), "clone", "--bare", source, bare)
	first := filepath.Join(t.TempDir(), "first")
	if err := Clone(ctx, bare, first); err != nil {
		t.Fatal(err)
	}
	firstClient, err := Open(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	modulePath, _ := project.ModulePath(uuid.MustNew())
	if err := os.WriteFile(filepath.Join(first, filepath.FromSlash(modulePath)), []byte("Первый();\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configureIdentity(t, first)
	if _, err := firstClient.Commit(ctx, "Remote change"); err != nil {
		t.Fatal(err)
	}
	if status, err := firstClient.Status(ctx); err != nil || status.Ahead != 1 {
		t.Fatalf("ahead status=%+v error=%v", status, err)
	}
	if _, err := firstClient.Push(ctx); err != nil {
		t.Fatal(err)
	}
	second := filepath.Join(t.TempDir(), "second")
	if err := Clone(ctx, bare, second); err != nil {
		t.Fatal(err)
	}
	secondClient, err := Open(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := firstClient.Pull(ctx); !errors.Is(err, ErrDirtyWorktree) {
		t.Fatalf("dirty pull error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(second, "README.md"), []byte("from second\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	configureIdentity(t, second)
	if _, err := secondClient.Commit(ctx, "Second change"); err != nil {
		t.Fatal(err)
	}
	if _, err := secondClient.Push(ctx); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(first, "README.md"), []byte("# MetaLab test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := firstClient.Pull(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestOpenRejectsParentRepositoryAndCloneCredentials(t *testing.T) {
	requireGit(t)
	parent := t.TempDir()
	runGit(t, parent, "init", "-b", "main")
	child := filepath.Join(parent, "project")
	initializeProject(t, child)
	if _, err := Open(context.Background(), child); !errors.Is(err, ErrUnsafeRepository) {
		t.Fatalf("Open() error = %v", err)
	}
	if err := Clone(context.Background(), "https://token@example.com/repository.git", filepath.Join(t.TempDir(), "clone")); err == nil || !strings.Contains(err.Error(), "credentials") {
		t.Fatalf("Clone() credential error = %v", err)
	}
	if value := redactCredentials("fatal: https://secret@example.com/repository.git"); strings.Contains(value, "secret") {
		t.Fatalf("credentials were not redacted: %s", value)
	}
}

func TestParseStatusPreservesRenameConflictAndTracking(t *testing.T) {
	status, err := parseStatus("## main...origin/main [ahead 2, behind 1]\x00R  new name.yaml\x00old name.yaml\x00UU conflict.bsl\x00")
	if err != nil {
		t.Fatal(err)
	}
	if status.Branch != "main" || status.Ahead != 2 || status.Behind != 1 || len(status.Entries) != 2 {
		t.Fatalf("status = %+v", status)
	}
	if status.Entries[0].Path != "new name.yaml" || status.Entries[0].OriginalPath != "old name.yaml" || !status.Entries[1].Conflicted {
		t.Fatalf("entries = %+v", status.Entries)
	}
}

func initializedRepository(t *testing.T) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "project")
	initializeProject(t, root)
	runGit(t, root, "init", "-b", "main")
	configureIdentity(t, root)
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("# MetaLab test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "--all")
	runGit(t, root, "commit", "-m", "Initial project")
	return root
}

func initializeProject(t *testing.T, root string) {
	t.Helper()
	manifest := project.Project{
		Format: project.CurrentFormat, ID: uuid.MustNew(), Name: "GitDemo", Title: "Git Demo",
		DefaultLanguage: "ru", Languages: []project.Language{{Name: "Русский", Title: "Русский", Code: "ru"}},
	}
	if err := project.Initialize(root, manifest); err != nil {
		t.Fatal(err)
	}
}

func configureIdentity(t *testing.T, root string) {
	t.Helper()
	runGit(t, root, "config", "user.name", "MetaLab Test")
	runGit(t, root, "config", "user.email", "metalab-test@example.invalid")
}

func runGit(t *testing.T, root string, arguments ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", root}, arguments...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", arguments, err, output)
	}
	return string(output)
}

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("Git is unavailable")
	}
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}
