package config

import (
	"os"
	"path/filepath"
	"strings"
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
		"OC_REG_OC_URL", "https://cloud.example.com",
		"OC_REG_OC_ADMIN_PASSWORD", "secret",
		"OC_REG_ADMIN_TOKEN", "token123",
	)
}

func TestLoad_RequiredFields(t *testing.T) {
	cases := []struct {
		name    string
		unset   string
		wantErr string
	}{
		{"missing OC_URL", "OC_REG_OC_URL", "OC_REG_OC_URL is required"},
		{"missing OC_ADMIN_PASSWORD", "OC_REG_OC_ADMIN_PASSWORD", "OC_REG_OC_ADMIN_PASSWORD is required"},
		{"missing ADMIN_TOKEN", "OC_REG_ADMIN_TOKEN", "OC_REG_ADMIN_TOKEN is required"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			validEnv(t)
			os.Unsetenv(tc.unset)

			_, err := Load()
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
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
		"OC_REG_OC_URL", "https://cloud.example.com",
		"OC_REG_OC_ADMIN_PASSWORD", "pass",
		"OC_REG_ADMIN_TOKEN", "tok",
		"OC_REG_LISTEN_ADDR", ":9090",
		"OC_REG_DB_PATH", "/tmp/test.db",
		"OC_REG_REGISTRATION_MODE", "approval",
		"OC_REG_OC_ADMIN_USER", "superadmin",
		"OC_REG_OC_INSECURE", "true",
		"OC_REG_APP_BASE_URL", "https://register.example.com",
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
	t.Setenv("OC_REG_REGISTRATION_MODE", "bogus")

	_, err := Load()
	if err == nil {
		t.Fatal("expected error for invalid mode")
	}
}

func TestLoad_ConfigFile(t *testing.T) {
	validEnv(t)
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "registration.yaml")
	
	yamlContent := `
listen_addr: ":1234"
registration_mode: "approval"
template_dir: "/tmp/tpls"
`
	if err := os.WriteFile(configPath, []byte(yamlContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("OC_REG_CONFIG_PATH", configPath)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.ListenAddr != ":1234" {
		t.Errorf("expected :1234, got %q", cfg.ListenAddr)
	}
	if cfg.RegistrationMode != "approval" {
		t.Errorf("expected approval, got %q", cfg.RegistrationMode)
	}
	if cfg.TemplateDir != "/tmp/tpls" {
		t.Errorf("expected /tmp/tpls, got %q", cfg.TemplateDir)
	}
	if cfg.LoadedConfigPath != configPath {
		t.Errorf("expected LoadedConfigPath %q, got %q", configPath, cfg.LoadedConfigPath)
	}

	// Test environment override
	t.Setenv("OC_REG_LISTEN_ADDR", ":5678")
	cfg2, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg2.ListenAddr != ":5678" {
		t.Errorf("env should override yaml, expected :5678, got %q", cfg2.ListenAddr)
	}
}

func TestObfuscate(t *testing.T) {
	cfg := &Config{
		AdminToken:      "super_secret_token",
		OCAdminPassword: "super_secret_password",
		ListenAddr:      ":8080",
	}

	obs := cfg.Obfuscate()

	if obs["AdminToken"] != "***" {
		t.Errorf("AdminToken not obfuscated: %v", obs["AdminToken"])
	}
	if obs["OCAdminPassword"] != "***" {
		t.Errorf("OCAdminPassword not obfuscated: %v", obs["OCAdminPassword"])
	}
	if obs["ListenAddr"] != ":8080" {
		t.Errorf("ListenAddr should not be obfuscated: %v", obs["ListenAddr"])
	}
}
