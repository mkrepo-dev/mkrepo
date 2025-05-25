package config

import (
	"bytes"
	"fmt"
	"os"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BaseUrl string `yaml:"baseUrl"`

	DatabaseUri  string `yaml:"databaseUri"`
	SecretKey    string `yaml:"secretKey"`
	MetricsToken string `yaml:"metricsToken"`

	WebhookSecret   string `yaml:"webhookSecret"`
	WebhookInsecure bool   `yaml:"webhookInsecure"`

	Providers []Provider `yaml:"providers"`
}

type Provider struct {
	Key          string       `yaml:"key"`
	Name         string       `yaml:"name"`
	Type         ProviderType `yaml:"type"`
	ClientID     string       `yaml:"clientId"`
	ClientSecret string       `yaml:"clientSecret"`
	Url          string       `yaml:"url"`
	ApiUrl       string       `yaml:"apiUrl"`
}

type ProviderType string

const (
	GitHubProvider ProviderType = "github"
	GitLabProvider ProviderType = "gitlab"
	GiteaProvider  ProviderType = "gitea"
)

var providerTypes = []ProviderType{GitHubProvider, GitLabProvider, GiteaProvider}

func LoadConfig(filename string) (Config, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	err = yaml.NewDecoder(bytes.NewReader(file)).Decode(&cfg)
	if err != nil {
		return Config{}, err
	}

	cfg = setDefaults(cfg)

	// Reencode so we can replace env vars with os.ExpandEnv
	var buff bytes.Buffer
	err = yaml.NewEncoder(&buff).Encode(cfg)
	if err != nil {
		return Config{}, err
	}
	expanded := os.ExpandEnv(buff.String())
	err = yaml.NewDecoder(strings.NewReader(expanded)).Decode(&cfg)
	if err != nil {
		return Config{}, err
	}

	err = validate(cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(cfg Config) Config {
	if cfg.BaseUrl == "" {
		cfg.BaseUrl = "http://localhost:8080"
	}
	if cfg.DatabaseUri == "" {
		cfg.DatabaseUri = "postgres://mkrepo:mkrepo@localhost:5432/mkrepo?sslmode=disable&search_path=public"
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = "$SECRET_KEY"
	}
	if cfg.MetricsToken == "" {
		cfg.MetricsToken = "$METRICS_TOKEN"
	}
	if cfg.WebhookSecret == "" {
		cfg.WebhookSecret = "$WEBHOOK_SECRET"
	}
	for i, provider := range cfg.Providers {
		cfg.Providers[i] = setDefaultsProvider(provider)
	}

	return cfg
}

func setDefaultsProvider(provider Provider) Provider {
	if provider.Type == "" && slices.Contains(providerTypes, ProviderType(provider.Key)) {
		provider.Type = ProviderType(provider.Key)
	}
	if provider.Name == "" {
		switch provider.Type {
		case GitHubProvider:
			provider.Name = "GitHub"
		case GitLabProvider:
			provider.Name = "GitLab"
		case GiteaProvider:
			provider.Name = "Gitea"
		}
	}
	return provider
}

func validate(cfg Config) error {
	if cfg.SecretKey == "" {
		return fmt.Errorf("missing secret key")
	}

	if len(cfg.Providers) == 0 {
		return fmt.Errorf("no providers defined")
	}
	keys := make(map[string]struct{})
	for _, provider := range cfg.Providers {
		if _, ok := keys[provider.Key]; ok {
			return fmt.Errorf("duplicate provider key: %s", provider.Key)
		}
		keys[provider.Key] = struct{}{}
	}
	for _, provider := range cfg.Providers {
		err := validateProvider(provider)
		if err != nil {
			return fmt.Errorf("invalid provider %s: %w", provider.Key, err)
		}
	}

	return nil
}

func validateProvider(provider Provider) error {
	if provider.Key == "" {
		return fmt.Errorf("missing provider key")
	}
	if provider.ClientID == "" {
		return fmt.Errorf("missing client ID")
	}
	if provider.ClientSecret == "" {
		return fmt.Errorf("missing client secret")
	}
	if !slices.Contains(providerTypes, provider.Type) {
		return fmt.Errorf("unknown provider type: %s", provider.Type)
	}
	return nil
}
