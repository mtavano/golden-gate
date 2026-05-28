package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type ServiceConfig struct {
	BasePrefix string `json:"base_prefix"`
	Target     string `json:"target"`
}

type Config struct {
	Services map[string]ServiceConfig `json:"-"`
}

// LoadConfig reads the service.json at the given path. If the file does not
// exist, it bootstraps the path with the embedded default and loads that.
func LoadConfig(configPath string) (*Config, error) {
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := bootstrapConfig(configPath); err != nil {
			return nil, fmt.Errorf("config bootstrap: %w", err)
		}
	} else if err != nil {
		return nil, err
	}

	file, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := &Config{Services: map[string]ServiceConfig{}}
	if err := json.Unmarshal(file, &cfg.Services); err != nil {
		return nil, fmt.Errorf("config parse %s: %w", configPath, err)
	}
	if err := Validate(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// bootstrapConfig writes the embedded default service.json to configPath,
// creating parent directories as needed.
func bootstrapConfig(configPath string) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath, DefaultServiceJSON, 0o644)
}

// Validate rejects configurations that would conflict with the dashboard
// routes or contain malformed entries. It returns a multi-error string so
// every offending service is surfaced at once.
func Validate(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}

	var errs []string
	seenPrefix := make(map[string]string, len(cfg.Services))

	for name, sc := range cfg.Services {
		if name == "" {
			errs = append(errs, "service has empty name")
			continue
		}
		if sc.BasePrefix == "" {
			errs = append(errs, fmt.Sprintf("%s: base_prefix is empty", name))
		} else if !strings.HasPrefix(sc.BasePrefix, "/") {
			errs = append(errs, fmt.Sprintf("%s: base_prefix %q must start with /", name, sc.BasePrefix))
		} else if sc.BasePrefix == "/" {
			errs = append(errs, fmt.Sprintf("%s: base_prefix '/' would shadow the dashboard", name))
		} else if strings.HasPrefix(sc.BasePrefix, "/dashboard") {
			errs = append(errs, fmt.Sprintf("%s: base_prefix %q is reserved (collides with /dashboard)", name, sc.BasePrefix))
		} else if other, ok := seenPrefix[sc.BasePrefix]; ok {
			errs = append(errs, fmt.Sprintf("%s: base_prefix %q is already used by %s", name, sc.BasePrefix, other))
		} else {
			seenPrefix[sc.BasePrefix] = name
		}

		if sc.Target == "" {
			errs = append(errs, fmt.Sprintf("%s: target is empty", name))
		} else {
			u, err := url.Parse(sc.Target)
			if err != nil {
				errs = append(errs, fmt.Sprintf("%s: target %q is not a valid URL: %v", name, sc.Target, err))
			} else if u.Scheme != "http" && u.Scheme != "https" {
				errs = append(errs, fmt.Sprintf("%s: target %q must use http or https", name, sc.Target))
			} else if u.Host == "" {
				errs = append(errs, fmt.Sprintf("%s: target %q has no host", name, sc.Target))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid service config:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}
