package buildinfo

import "testing"

func TestInfoString(t *testing.T) {
	t.Parallel()

	info := Info{Version: "1.2.3", Commit: "abc123", Date: "2026-09-01"}
	want := "MetaLab 1.2.3 (commit: abc123, built: 2026-09-01)"

	if got := info.String(); got != want {
		t.Fatalf("Info.String() = %q, want %q", got, want)
	}
}
