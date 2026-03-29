package config

import (
	"errors"
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ListenAddr       string `yaml:"listen_addr"`
	DBPath           string `yaml:"db_path"`
	RegistrationMode string `yaml:"registration_mode"` // "open" | "approval"
	AdminToken       string `yaml:"admin_token"`
	OCUrl            string `yaml:"oc_url"`
	OCAdminUser      string `yaml:"oc_admin_user"`
	OCAdminPassword  string `yaml:"oc_admin_password"`
	OCInsecure       bool   `yaml:"oc_insecure"`
	AppBaseURL       string `yaml:"app_base_url"`
	TemplateDir      string `yaml:"template_dir"`
}

func Load() (*Config, error) {
	// 1. Set Defaults
	cfg := &Config{
		ListenAddr:       ":8080",
		DBPath:           "/data/registration.db",
		RegistrationMode: "open",
		OCAdminUser:      "admin",
		AppBaseURL:       "http://localhost:8080",
	}

	// 2. Load from YAML if exists
	configPath := getEnv("CONFIG_PATH", "/data/config.yml")
	if _, err := os.Stat(configPath); err == nil {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, errors.New("could not read config file: " + err.Error())
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, errors.New("could not parse config file: " + err.Error())
		}
	}

	// 3. Environment Variables (Highest Precedence)
	if v := os.Getenv("LISTEN_ADDR"); v != "" {
		cfg.ListenAddr = v
	}
	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}
	if v := os.Getenv("REGISTRATION_MODE"); v != "" {
		cfg.RegistrationMode = v
	}
	if v := os.Getenv("ADMIN_TOKEN"); v != "" {
		cfg.AdminToken = v
	}
	if v := os.Getenv("OC_URL"); v != "" {
		cfg.OCUrl = v
	}
	if v := os.Getenv("OC_ADMIN_USER"); v != "" {
		cfg.OCAdminUser = v
	}
	if v := os.Getenv("OC_ADMIN_PASSWORD"); v != "" {
		cfg.OCAdminPassword = v
	}
	if v := os.Getenv("OC_INSECURE"); v != "" {
		cfg.OCInsecure = parseBool(v)
	}
	if v := os.Getenv("APP_BASE_URL"); v != "" {
		cfg.AppBaseURL = v
	}
	if v := os.Getenv("TEMPLATE_DIR"); v != "" {
		cfg.TemplateDir = v
	}

	// 4. Validate
	var errs []string
	if cfg.OCUrl == "" {
		errs = append(errs, "OC_URL is required")
	}
	if cfg.OCAdminPassword == "" {
		errs = append(errs, "OC_ADMIN_PASSWORD is required")
	}
	if cfg.AdminToken == "" {
		errs = append(errs, "ADMIN_TOKEN is required")
	}
	if cfg.RegistrationMode != "open" && cfg.RegistrationMode != "approval" {
		errs = append(errs, "REGISTRATION_MODE must be 'open' or 'approval'")
	}

	if len(errs) > 0 {
		msg := "config errors:"
		for _, e := range errs {
			msg += "\n  - " + e
		}
		return nil, errors.New(msg)
	}

	return cfg, nil
}

func (c *Config) Obfuscate() map[string]any {
	return map[string]any{
		"ListenAddr":       c.ListenAddr,
		"DBPath":           c.DBPath,
		"RegistrationMode": c.RegistrationMode,
		"AdminToken":       "***",
		"OCUrl":            c.OCUrl,
		"OCAdminUser":      c.OCAdminUser,
		"OCAdminPassword":  "***",
		"OCInsecure":       c.OCInsecure,
		"AppBaseURL":       c.AppBaseURL,
		"TemplateDir":      c.TemplateDir,
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
