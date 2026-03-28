package config

import (
	"os"
	"testing"
)

func setEnv(t *testing.T, pairs ...string) {
	t.Helper()
	for i := 0; i+1 < len(pairs); i += 2 {
		t.Setenv(pairs[i], pairs[i+1])
	}
}

func validEnv(t *testing.T) {
	t.Helper()
	setEnv(t,
		"OC_URL", "https://cloud.example.com",
		"OC_ADMIN_PASSWORD", "secret",
		"ADMIN_TOKEN", "token123",
	)
}

func TestLoad_RequiredFields(t *testing.T) {
	cases := []struct {
		name    string
		unset   string
		wantErr string
	}{
		{"missing OC_URL", "OC_URL", "OC_URL is required"},
		{"missing OC_ADMIN_PASSWORD", "OC_ADMIN_PASSWORD", "OC_ADMIN_PASSWORD is required"},
		{"missing ADMIN_TOKEN", "ADMIN_TOKEN", "ADMIN_TOKEN is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			os.Unsetenv(tc.unset)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !contains(err.Error(), tc.wantErr) {
				t.Errorf("expected error to contain %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestLoad_Defaults(t *testing.T) {
	validEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr: got %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.DBPath != "/data/registration.db" {
		t.Errorf("DBPath: got %q", cfg.DBPath)
	}
	if cfg.RegistrationMode != "open" {
		t.Errorf("RegistrationMode: got %q", cfg.RegistrationMode)
	}
	if cfg.OCAdminUser != "admin" {
		t.Errorf("OCAdminUser: got %q", cfg.OCAdminUser)
	}
	if cfg.OCInsecure {
		t.Error("OCInsecure: expected false")
	}
}

func TestLoad_AllSet(t *testing.T) {
	setEnv(t,
		"OC_URL", "https://cloud.example.com",
		"OC_ADMIN_PASSWORD", "pass",
		"ADMIN_TOKEN", "tok",
		"LISTEN_ADDR", ":9090",
		"DB_PATH", "/tmp/test.db",
		"REGISTRATION_MODE", "approval",
		"OC_ADMIN_USER", "superadmin",
		"OC_INSECURE", "true",
		"APP_BASE_URL", "https://register.example.com",
	)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr: got %q", cfg.ListenAddr)
	}
	if cfg.RegistrationMode != "approval" {
		t.Errorf("RegistrationMode: got %q", cfg.RegistrationMode)
	}
	if !cfg.OCInsecure {
		t.Error("OCInsecure: expected true")
	}
	if cfg.AppBaseURL != "https://register.example.com" {
		t.Errorf("AppBaseURL: got %q", cfg.AppBaseURL)
	}
}

func TestLoad_InvalidMode(t *testing.T) {
	validEnv(t)
	t.Setenv("REGISTRATION_MODE", "bogus")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
