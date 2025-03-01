package config

import (
	"fmt"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Providers []Provider `yaml:"providers"`
}

type Provider struct {
	Key          string       `yaml:"key"`
	Name         string       `yaml:"name"`
	Type         ProviderType `yaml:"type"`
	ClientID     string       `yaml:"clientId"`
	ClientSecret string       `yaml:"clientSecret"`
	Url          string       `yaml:"url"`
}

type ProviderType string

const (
	GitHubProvider ProviderType = "github"
	GitLabProvider ProviderType = "gitlab"
)

var providerDefaults = map[ProviderType]Provider{
	GitHubProvider: {
		Key:  string(GitHubProvider),
		Name: "GitHub",
	},
	GitLabProvider: {
		Key:  string(GitLabProvider),
		Name: "GitLab",
	},
}

func LoadConfig(filename string) (Config, error) {
	vp := viper.NewWithOptions(viper.WithLogger(slog.Default()))
	vp.SetConfigFile(filename)

	err := vp.ReadInConfig()
	if err != nil {
		return Config{}, err
	}

	var config Config
	err = vp.Unmarshal(&config)
	if err != nil {
		return Config{}, err
	}

	config = setDefaults(config)
	err = validate(config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}

func setDefaults(cfg Config) Config {
	for i, provider := range cfg.Providers {
		cfg.Providers[i] = setDefaultsProvider(provider)
	}

	return cfg
}

func setDefaultsProvider(provider Provider) Provider {
	if provider.Key == "" {
		provider.Key = providerDefaults[provider.Type].Key
	}
	if provider.Name == "" {
		provider.Name = providerDefaults[provider.Type].Name
	}
	provider.ClientID = readFromEnv(provider.ClientID)
	provider.ClientSecret = readFromEnv(provider.ClientSecret)
	return provider
}

func validate(cfg Config) error {
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
	if provider.Name == "" {
		return fmt.Errorf("missing provider name")
	}
	if provider.ClientID == "" {
		return fmt.Errorf("missing client ID")
	}
	if provider.ClientSecret == "" {
		return fmt.Errorf("missing client secret")
	}
	if !slices.Contains([]ProviderType{GitHubProvider, GitLabProvider}, provider.Type) {
		return fmt.Errorf("unknown provider type: %s", provider.Type)
	}
	return nil
}

func readFromEnv(key string) string {
	if key, ok := strings.CutPrefix(key, "$"); ok {
		return os.Getenv(key)
	}
	return key
}
