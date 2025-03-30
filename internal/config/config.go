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
	BaseUrl   string     `yaml:"baseUrl"`
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
)

var ProviderTypes = []ProviderType{GitHubProvider, GitLabProvider}

func LoadConfig(filename string) (Config, error) {
	vp := viper.NewWithOptions(viper.WithLogger(slog.Default()))
	vp.SetConfigFile(filename)

	err := vp.ReadInConfig()
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	err = vp.Unmarshal(&cfg)
	if err != nil {
		return Config{}, err
	}

	cfg = setDefaults(cfg)
	cfg = fillFromEnv(cfg)
	err = validate(cfg)
	if err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func setDefaults(cfg Config) Config {
	for i, provider := range cfg.Providers {
		cfg.Providers[i] = setDefaultsProvider(provider)
	}

	return cfg
}

func setDefaultsProvider(provider Provider) Provider {
	if provider.Type == "" && slices.Contains(ProviderTypes, ProviderType(provider.Key)) {
		provider.Type = ProviderType(provider.Key)
	}
	if provider.ClientID == "" {
		provider.ClientID = fmt.Sprintf("$%s_CLIENT_ID", strings.ToUpper(provider.Key))
	}
	if provider.ClientSecret == "" {
		provider.ClientSecret = fmt.Sprintf("$%s_CLIENT_SECRET", strings.ToUpper(provider.Key))
	}
	return provider
}

func fillFromEnv(cfg Config) Config {
	for i, provider := range cfg.Providers {
		cfg.Providers[i].ClientID = readFromEnv(provider.ClientID)
		cfg.Providers[i].ClientSecret = readFromEnv(provider.ClientSecret)
	}
	return cfg
}

func validate(cfg Config) error {
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
	if !slices.Contains(ProviderTypes, provider.Type) {
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
