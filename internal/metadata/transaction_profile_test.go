package metadata

import (
	"context"
	"testing"
)

func TestTestExecutionProfileIsFailClosed(t *testing.T) {
	ctx := context.WithValue(context.Background(), testExecutionContextKey{}, TestExecutionProfile{Role: "Менеджер"})
	profile, ok := TestExecutionProfileFromContext(ctx)
	if !ok || profile.Role != "Менеджер" || profile.AllowExternalCalls || profile.AllowEquipmentAccess {
		t.Fatalf("profile=%+v ok=%v", profile, ok)
	}
	if _, ok := TestExecutionProfileFromContext(context.Background()); ok {
		t.Fatal("ordinary execution has a test profile")
	}
}
