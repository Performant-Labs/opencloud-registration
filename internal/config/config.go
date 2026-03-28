package config

import (
	"errors"
	"os"
	"strconv"
)

type Config struct {
	ListenAddr       string
	DBPath           string
	RegistrationMode string // "open" | "approval"
	AdminToken       string
	OCUrl            string
	OCAdminUser      string
	OCAdminPassword  string
	OCInsecure       bool
	AppBaseURL       string
}

func Load() (*Config, error) {
	cfg := &Config{
		ListenAddr:       getEnv("LISTEN_ADDR", ":8080"),
		DBPath:           getEnv("DB_PATH", "/data/registration.db"),
		RegistrationMode: getEnv("REGISTRATION_MODE", "open"),
		AdminToken:       os.Getenv("ADMIN_TOKEN"),
		OCUrl:            os.Getenv("OC_URL"),
		OCAdminUser:      getEnv("OC_ADMIN_USER", "admin"),
		OCAdminPassword:  os.Getenv("OC_ADMIN_PASSWORD"),
		OCInsecure:       parseBool(os.Getenv("OC_INSECURE")),
		AppBaseURL:       getEnv("APP_BASE_URL", "http://localhost:8080"),
	}

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
