package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	PollInterval       string        `yaml:"poll_interval"`
	SupportWindowDays  int           `yaml:"support_window_days"`
	Repos              []RepoConfig  `yaml:"repos"`
	Notifications      Notifications `yaml:"notifications"`
	EnableChangelog    bool          `yaml:"enable_changelog"`
	EnableHealthChecks bool          `yaml:"enable_health_checks"`
	ChangelogKeywords  []string      `yaml:"changelog_keywords"`
}

// RepoConfig represents a single repository to monitor
type RepoConfig struct {
	Owner            string   `yaml:"owner"`
	Repo             string   `yaml:"repo"`
	Type             string   `yaml:"type"` // binary|image|helm
	PinnedVersion    string   `yaml:"pinned_version"`
	CriticalKeywords []string `yaml:"critical_keywords"`
}

// Notifications contains notification channel configurations
type Notifications struct {
	GoogleChat GoogleChatConfig `yaml:"google_chat"`
	Email      EmailConfig      `yaml:"email"`
}

// GoogleChatConfig contains Google Chat webhook configuration
type GoogleChatConfig struct {
	WebhookURL string `yaml:"webhook_url"`
	Space      string `yaml:"space"`
}

// EmailConfig contains email notification configuration
type EmailConfig struct {
	SMTPHost string   `yaml:"smtp_host"`
	From     string   `yaml:"from"`
	To       []string `yaml:"to"`
}

// loadConfig reads and parses the configuration file
func loadConfig(path string) (Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}
	var cfg Config
	if err := yaml.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.SupportWindowDays == 0 {
		cfg.SupportWindowDays = 30
	}
	if cfg.PollInterval == "" {
		cfg.PollInterval = "10m"
	}
	return cfg, nil
}
