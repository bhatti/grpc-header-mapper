package headermapper

import (
	"encoding/json"
	"fmt"
	"io"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadConfigFromFile loads configuration from a file (JSON or YAML)
func LoadConfigFromFile(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	
	// Try YAML first, then JSON
	if err := yaml.Unmarshal(data, &config); err != nil {
		if err := json.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file as YAML or JSON: %w", err)
		}
	}

	return &config, nil
}

// SaveConfigToFile saves configuration to a file
func SaveConfigToFile(config *Config, filename string, format string) error {
	var data []byte
	var err error

	switch format {
	case "yaml", "yml":
		data, err = yaml.Marshal(config)
	case "json":
		data, err = json.MarshalIndent(config, "", "  ")
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}

	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(filename, data, 0644)
}

// ConfigBuilder helps build configurations programmatically
type ConfigBuilder struct {
	config *Config
}

// NewConfigBuilder creates a new configuration builder
func NewConfigBuilder() *ConfigBuilder {
	return &ConfigBuilder{
		config: &Config{
			Mappings: make([]HeaderMapping, 0),
		},
	}
}

// WithMappings sets the header mappings
func (cb *ConfigBuilder) WithMappings(mappings []HeaderMapping) *ConfigBuilder {
	cb.config.Mappings = mappings
	return cb
}

// AddMapping adds a single header mapping
func (cb *ConfigBuilder) AddMapping(mapping HeaderMapping) *ConfigBuilder {
	cb.config.Mappings = append(cb.config.Mappings, mapping)
	return cb
}

// WithSkipPaths sets the paths to skip
func (cb *ConfigBuilder) WithSkipPaths(paths []string) *ConfigBuilder {
	cb.config.SkipPaths = paths
	return cb
}

// WithCaseSensitive sets case sensitivity
func (cb *ConfigBuilder) WithCaseSensitive(caseSensitive bool) *ConfigBuilder {
	cb.config.CaseSensitive = caseSensitive
	return cb
}

// WithOverwriteExisting sets overwrite behavior
func (cb *ConfigBuilder) WithOverwriteExisting(overwrite bool) *ConfigBuilder {
	cb.config.OverwriteExisting = overwrite
	return cb
}

// WithDebug sets debug mode
func (cb *ConfigBuilder) WithDebug(debug bool) *ConfigBuilder {
	cb.config.Debug = debug
	return cb
}

// Build returns the built configuration
func (cb *ConfigBuilder) Build() *Config {
	return cb.config
}

// ValidateConfig performs comprehensive configuration validation
func ValidateConfig(config *Config) error {
	if config == nil {
		return fmt.Errorf("configuration is nil")
	}

	// Check for duplicate mappings
	seen := make(map[string]HeaderMapping)
	for i, mapping := range config.Mappings {
		if mapping.HTTPHeader == "" {
			return fmt.Errorf("mapping %d: HTTPHeader cannot be empty", i)
		}
		if mapping.GRPCMetadata == "" {
			return fmt.Errorf("mapping %d: GRPCMetadata cannot be empty", i)
		}

		key := fmt.Sprintf("%s->%s", mapping.HTTPHeader, mapping.GRPCMetadata)
		if existing, exists := seen[key]; exists {
			return fmt.Errorf("duplicate mapping found: %s (directions: %d, %d)", 
				key, existing.Direction, mapping.Direction)
		}
		seen[key] = mapping
	}

	return nil
}
