package metadata

import (
	"context"
	"testing"
)

func TestBSLEventGuardRejectsCyclesAndAllowsNestedHandlers(t *testing.T) {
	t.Parallel()
	first, second := new(int), new(int)
	ctx, err := enterBSLEvent(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	ctx, err = enterBSLEvent(ctx, second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := enterBSLEvent(ctx, first); err == nil {
		t.Fatal("recursive BSL event chain was accepted")
	}
}
