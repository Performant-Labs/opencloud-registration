package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type Config struct {
	ListenAddr       string `mapstructure:"listen_addr"`
	DBPath           string `mapstructure:"db_path"`
	RegistrationMode string `mapstructure:"registration_mode"` // "open" | "approval"
	AdminToken       string `mapstructure:"admin_token"`
	OCUrl            string `mapstructure:"oc_url"`
	OCAdminUser      string `mapstructure:"oc_admin_user"`
	OCAdminPassword  string `mapstructure:"oc_admin_password"`
	OCInsecure       bool   `mapstructure:"oc_insecure"`
	AppBaseURL       string `mapstructure:"app_base_url"`
	TemplateDir      string `mapstructure:"template_dir"`

	LoadedConfigPath string `mapstructure:"-"` // Not loaded from config
}

func Load() (*Config, error) {
	v := viper.New()

	v.SetConfigName("registration")

	if envPath := os.Getenv("OC_REG_CONFIG_PATH"); envPath != "" {
		v.SetConfigFile(envPath)
	} else {
		v.AddConfigPath("/data")
		v.AddConfigPath("/etc/opencloud-registration/")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(home + "/.opencloud-registration/")
		}
		v.AddConfigPath(".")
	}

	// Track all keys so Unmarshal can read from env vars
	v.SetDefault("listen_addr", ":8080")
	v.SetDefault("db_path", "/data/registration.db")
	v.SetDefault("registration_mode", "open")
	v.SetDefault("admin_token", "")
	v.SetDefault("oc_url", "")
	v.SetDefault("oc_admin_user", "admin")
	v.SetDefault("oc_admin_password", "")
	v.SetDefault("oc_insecure", false)
	v.SetDefault("app_base_url", "http://localhost:8080")
	v.SetDefault("template_dir", "")

	v.SetEnvPrefix("OC_REG")
	v.AutomaticEnv()

	var cfg Config
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("could not read config file: %w", err)
		}
	} else {
		cfg.LoadedConfigPath = v.ConfigFileUsed()
	}

	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("could not parse config: %w", err)
	}

	var errs []string
	if cfg.OCUrl == "" {
		errs = append(errs, "OC_REG_OC_URL is required")
	}
	if cfg.OCAdminPassword == "" {
		errs = append(errs, "OC_REG_OC_ADMIN_PASSWORD is required")
	}
	if cfg.AdminToken == "" {
		errs = append(errs, "OC_REG_ADMIN_TOKEN is required")
	}
	if cfg.RegistrationMode != "open" && cfg.RegistrationMode != "approval" {
		errs = append(errs, "OC_REG_REGISTRATION_MODE must be 'open' or 'approval'")
	}

	if len(errs) > 0 {
		msg := "config errors:"
		for _, e := range errs {
			msg += "\n  - " + e
		}
		return nil, errors.New(msg)
	}

	return &cfg, nil
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
		"LoadedConfigPath": c.LoadedConfigPath,
	}
}
