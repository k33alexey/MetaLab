package testsuite

import (
	"context"
	"errors"
	"fmt"

	"github.com/k33alexey/MetaLab/internal/gitclient"
)

var ErrGitConflict = errors.New("ML Project contains unresolved Git conflicts")

// EnsureConflictFree prevents testing a source tree containing conflict markers.
// A project not yet placed in Git remains testable locally.
func EnsureConflictFree(ctx context.Context, root string) error {
	client, err := gitclient.Open(ctx, root)
	if errors.Is(err, gitclient.ErrNotRepository) || errors.Is(err, gitclient.ErrUnsafeRepository) {
		return nil
	}
	if err != nil {
		return err
	}
	status, err := client.Status(ctx)
	if err != nil {
		return err
	}
	if path := conflictPath(status); path != "" {
		return fmt.Errorf("%w: %s", ErrGitConflict, path)
	}
	return nil
}

func conflictPath(status gitclient.Status) string {
	for _, entry := range status.Entries {
		if entry.Conflicted {
			return entry.Path
		}
	}
	return ""
}
