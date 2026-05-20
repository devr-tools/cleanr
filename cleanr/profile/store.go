package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	homeEnvName   = "CLEANR_HOME"
	profileFile   = "profile.json"
	profileSchema = "v1alpha1"
)

type File struct {
	Version         string              `json:"version"`
	DefaultProvider string              `json:"default_provider"`
	UpdatedAt       time.Time           `json:"updated_at"`
	Providers       map[string]Provider `json:"providers"`
}

type Provider struct {
	Name         string    `json:"name"`
	Model        string    `json:"model"`
	APIMode      string    `json:"api_mode,omitempty"`
	APIKeyEnv    string    `json:"api_key_env"`
	APIKey       string    `json:"api_key"`
	BaseURL      string    `json:"base_url,omitempty"`
	Organization string    `json:"organization,omitempty"`
	Project      string    `json:"project,omitempty"`
	MaxTokens    int       `json:"max_tokens,omitempty"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func Load() (File, error) {
	path, err := profilePath()
	if err != nil {
		return File{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return File{}, err
	}

	var file File
	if err := json.Unmarshal(data, &file); err != nil {
		return File{}, fmt.Errorf("decode profile: %w", err)
	}
	normalizeFile(&file)
	return file, nil
}

func LoadOrCreate() (File, error) {
	file, err := Load()
	if err == nil {
		return file, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return defaultFile(), nil
	}
	return File{}, err
}

func Save(file File) error {
	normalizeFile(&file)
	file.UpdatedAt = time.Now().UTC()

	path, err := profilePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create profile dir: %w", err)
	}

	data, err := json.MarshalIndent(file, "", "  ")
	if err != nil {
		return fmt.Errorf("encode profile: %w", err)
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func UpsertProvider(provider Provider) error {
	file, err := LoadOrCreate()
	if err != nil {
		return err
	}

	key := normalizeProviderName(provider.Name)
	if key == "" {
		return fmt.Errorf("provider name is required")
	}

	provider.Name = key
	provider.Model = strings.TrimSpace(provider.Model)
	provider.APIMode = strings.TrimSpace(provider.APIMode)
	provider.APIKeyEnv = strings.TrimSpace(provider.APIKeyEnv)
	provider.APIKey = strings.TrimSpace(provider.APIKey)
	provider.BaseURL = strings.TrimSpace(provider.BaseURL)
	provider.Organization = strings.TrimSpace(provider.Organization)
	provider.Project = strings.TrimSpace(provider.Project)
	provider.UpdatedAt = time.Now().UTC()

	file.Providers[key] = provider
	file.DefaultProvider = key
	return Save(file)
}

func DefaultProvider() (Provider, error) {
	file, err := Load()
	if err != nil {
		return Provider{}, err
	}
	key := normalizeProviderName(file.DefaultProvider)
	if key == "" {
		return Provider{}, fmt.Errorf("no default provider is configured")
	}
	provider, ok := file.Providers[key]
	if !ok {
		return Provider{}, fmt.Errorf("default provider %q is not configured", key)
	}
	return provider, nil
}

func LookupAPIKey(providerName, envName string) (string, error) {
	file, err := Load()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", err
	}

	key := normalizeProviderName(providerName)
	if key == "" {
		key = normalizeProviderName(file.DefaultProvider)
	}
	provider, ok := file.Providers[key]
	if !ok {
		return "", nil
	}

	requestedEnv := strings.TrimSpace(envName)
	storedEnv := strings.TrimSpace(provider.APIKeyEnv)
	if requestedEnv != "" && storedEnv != "" && requestedEnv != storedEnv {
		return "", nil
	}
	return strings.TrimSpace(provider.APIKey), nil
}

func Path() (string, error) {
	return profilePath()
}

func defaultFile() File {
	return File{
		Version:   profileSchema,
		Providers: map[string]Provider{},
	}
}

func normalizeFile(file *File) {
	if file.Version == "" {
		file.Version = profileSchema
	}
	if file.Providers == nil {
		file.Providers = map[string]Provider{}
	}
	if file.DefaultProvider != "" {
		file.DefaultProvider = normalizeProviderName(file.DefaultProvider)
	}

	normalized := make(map[string]Provider, len(file.Providers))
	for key, provider := range file.Providers {
		name := normalizeProviderName(key)
		if provider.Name != "" {
			name = normalizeProviderName(provider.Name)
		}
		if name == "" {
			continue
		}
		provider.Name = name
		normalized[name] = provider
	}
	file.Providers = normalized
}

func profilePath() (string, error) {
	dir, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, profileFile), nil
}

func baseDir() (string, error) {
	if dir := strings.TrimSpace(os.Getenv(homeEnvName)); dir != "" {
		return dir, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".cleanr"), nil
}

func normalizeProviderName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
