package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Verbose bool   `mapstructure:"verbose"`
	JSON    bool   `mapstructure:"json"`
}

// Load loads configuration from file, environment variables, and flags
// Priority: CLI flags > Environment variables > Config file > Defaults
func Load() (*Config, error) {
	// Set default values
	viper.SetDefault("host", "localhost")
	viper.SetDefault("port", 80)
	viper.SetDefault("verbose", false)
	viper.SetDefault("json", false)

	// Set config file name and paths
	viper.SetConfigName("config")
	viper.SetConfigType("toml")

	// Add config paths to search
	// 1. ~/.config/c64u/config.toml (XDG standard)
	if homeDir, err := os.UserHomeDir(); err == nil {
		configDir := filepath.Join(homeDir, ".config", "c64u")
		viper.AddConfigPath(configDir)
	}

	// 2. Current directory
	viper.AddConfigPath(".")

	// Read config file (ignore if not found)
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Config file found but another error occurred
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
		// Config file not found, continue with defaults
	}

	// Environment variables
	viper.SetEnvPrefix("C64U")
	viper.AutomaticEnv()

	// Unmarshal config into struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return &cfg, nil
}

// CreateDefaultConfig creates a default config file in ~/.config/c64u/
func CreateDefaultConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(homeDir, ".config", "c64u")
	configPath := filepath.Join(configDir, "config.toml")

	// Check if config already exists
	if _, err := os.Stat(configPath); err == nil {
		return fmt.Errorf("config file already exists at: %s", configPath)
	}

	// Create config directory
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Default config content
	defaultConfig := `# c64u Configuration File
# C64 Ultimate CLI Tool

# C64 Ultimate hostname or IP address
host = "localhost"

# HTTP port (default: 80)
port = 80

# Example for a specific C64 Ultimate on network:
# host = "192.168.1.100"
# port = 80
`

	// Write config file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfigPath returns the path to the config file if it exists
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(homeDir, ".config", "c64u", "config.toml")
}
