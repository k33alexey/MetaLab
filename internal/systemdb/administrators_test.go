package systemdb

import "testing"

func TestValidateAdministratorLogin(t *testing.T) {
	t.Parallel()

	for _, login := range []string{"admin", "alexey@example.com", "админ"} {
		if err := validateLogin(login); err != nil {
			t.Fatalf("validateLogin(%q): %v", login, err)
		}
	}
	for _, login := range []string{"", " admin", "admin\n"} {
		if err := validateLogin(login); err == nil {
			t.Fatalf("validateLogin(%q) succeeded", login)
		}
	}
}
