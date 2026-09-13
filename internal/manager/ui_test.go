package manager

import (
	"context"
	"os/exec"
	"testing"
	"time"
)

func TestManagerUISessionIsolation(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for Manager UI behavior tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	output, err := exec.CommandContext(ctx, node, "--test", "../../scripts/manager-ui.test.mjs").CombinedOutput()
	if err != nil {
		t.Fatalf("Manager UI tests: %v\n%s", err, output)
	}
}
