package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
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
	debug.Log("Setting default configuration values")
	// Set default values
	viper.SetDefault("host", "localhost")
	viper.SetDefault("port", 80)
	viper.SetDefault("verbose", false)
	viper.SetDefault("json", false)

	// Set exact config file path: ~/.config/c64u/config.toml
	configFile := ""
	if homeDir, err := os.UserHomeDir(); err == nil {
		configFile = filepath.Join(homeDir, ".config", "c64u", "config.toml")
		viper.SetConfigFile(configFile)
		debug.Log("Config file: %s", configFile)
	} else {
		debug.LogError("Failed to get home directory: %v", err)
	}

	// Read config file (ignore if not found)
	debug.Log("Attempting to read config file...")
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || errors.Is(err, os.ErrNotExist) {
			// Config file not found, continue with defaults
			debug.Log("No config file found, using defaults")
		} else {
			// Config file found but has errors (e.g. invalid TOML syntax)
			configFileUsed := viper.ConfigFileUsed()
			debug.LogError("Error reading config file: %v", err)
			debug.LogError("Config file that caused error: %s", configFileUsed)

			// Try to read the file content for debugging
			if configFileUsed != "" {
				if content, readErr := os.ReadFile(configFileUsed); readErr == nil {
					debug.Log("Config file content (%d bytes):", len(content))
					debug.Log("--- BEGIN CONFIG FILE ---")
					debug.Log("%s", string(content))
					debug.Log("--- END CONFIG FILE ---")
				} else {
					debug.LogError("Could not read config file for debugging: %v", readErr)
				}
			}

			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	} else {
		debug.Log("Config file loaded successfully: %s", viper.ConfigFileUsed())
	}

	// Environment variables
	viper.SetEnvPrefix("C64U")
	viper.AutomaticEnv()
	debug.Log("Environment prefix: C64U")

	// Check for environment variables
	if envHost := os.Getenv("C64U_HOST"); envHost != "" {
		debug.Log("Found environment variable C64U_HOST=%s", envHost)
	}
	if envPort := os.Getenv("C64U_PORT"); envPort != "" {
		debug.Log("Found environment variable C64U_PORT=%s", envPort)
	}

	// Unmarshal config into struct
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		debug.LogError("Failed to unmarshal config: %v", err)
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	debug.Log("Final config values: host=%s, port=%d, verbose=%v, json=%v", cfg.Host, cfg.Port, cfg.Verbose, cfg.JSON)

	return &cfg, nil
}

// CreateDefaultConfig creates a default config file in ~/.config/c64u/
func CreateDefaultConfig() error {
	debug.Log("CreateDefaultConfig called")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		debug.LogError("Failed to get home directory: %v", err)
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	debug.Log("Home directory: %s", homeDir)

	configDir := filepath.Join(homeDir, ".config", "c64u")
	configPath := filepath.Join(configDir, "config.toml")
	debug.Log("Target config directory: %s", configDir)
	debug.Log("Target config file: %s", configPath)

	// Check if config already exists
	if info, err := os.Stat(configPath); err == nil {
		debug.Log("Config file already exists: %s (size: %d bytes, mode: %s)", configPath, info.Size(), info.Mode())
		// Read existing content for debugging
		if content, readErr := os.ReadFile(configPath); readErr == nil {
			debug.Log("Existing config file content:")
			debug.Log("--- BEGIN EXISTING CONFIG ---")
			debug.Log("%s", string(content))
			debug.Log("--- END EXISTING CONFIG ---")
		}
		return fmt.Errorf("config file already exists at: %s", configPath)
	} else {
		debug.Log("Config file does not exist (stat error: %v)", err)
	}

	// Check if config directory exists
	if info, err := os.Stat(configDir); err == nil {
		debug.Log("Config directory already exists: %s (mode: %s)", configDir, info.Mode())
		// List directory contents
		if entries, readErr := os.ReadDir(configDir); readErr == nil {
			debug.Log("Directory contents (%d entries):", len(entries))
			for _, entry := range entries {
				info, _ := entry.Info()
				if info != nil {
					debug.Log("  - %s (size: %d, mode: %s)", entry.Name(), info.Size(), info.Mode())
				} else {
					debug.Log("  - %s", entry.Name())
				}
			}
		}
	} else {
		debug.Log("Config directory does not exist, will create: %s", configDir)
	}

	// Create config directory
	debug.Log("Creating config directory: %s", configDir)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		debug.LogError("Failed to create config directory: %v", err)
		return fmt.Errorf("failed to create config directory: %w", err)
	}
	debug.Log("Config directory created successfully")

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

	debug.Log("Writing default config (%d bytes)", len(defaultConfig))
	debug.Log("--- BEGIN DEFAULT CONFIG ---")
	debug.Log("%s", defaultConfig)
	debug.Log("--- END DEFAULT CONFIG ---")

	// Write config file
	if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
		debug.LogError("Failed to write config file: %v", err)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	debug.Log("Config file written successfully: %s", configPath)

	// Verify the written file
	if content, readErr := os.ReadFile(configPath); readErr == nil {
		debug.Log("Verification: read back %d bytes from config file", len(content))
	} else {
		debug.LogError("Verification failed: could not read back config file: %v", readErr)
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
