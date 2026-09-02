package systemdb

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateSetting(t *testing.T) {
	t.Parallel()

	if err := validateSetting("service.language", json.RawMessage(`"ru"`)); err != nil {
		t.Fatal(err)
	}
	invalid := []struct {
		key   string
		value string
	}{
		{"", `{}`},
		{" service.language", `{}`},
		{"service..language", `{}`},
		{"Service.language", `{}`},
		{"service.language", ``},
		{"service.language", `{invalid}`},
	}
	for _, test := range invalid {
		if err := validateSetting(test.key, json.RawMessage(test.value)); err == nil {
			t.Fatalf("validateSetting(%q, %q) succeeded", test.key, test.value)
		}
	}
}

func TestSettingNotFoundCanBeMatched(t *testing.T) {
	t.Parallel()

	err := errors.Join(ErrSettingNotFound, errors.New("example"))
	if !errors.Is(err, ErrSettingNotFound) {
		t.Fatal("ErrSettingNotFound is not matchable")
	}
}
