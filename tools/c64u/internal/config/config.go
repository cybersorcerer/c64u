package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
	"github.com/spf13/viper"
)

// Device is one named C64 Ultimate in the config file.
type Device struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

// Config holds the application configuration.
//
// Host and Port are the resolved values for the device this invocation talks
// to. Devices and Default carry the file's contents so that `cli-config show`
// can list them and so an unknown --device can report what does exist.
type Config struct {
	Host    string `mapstructure:"host"`
	Port    int    `mapstructure:"port"`
	Verbose bool   `mapstructure:"verbose"`
	JSON    bool   `mapstructure:"json"`

	Devices map[string]Device `mapstructure:"devices"`
	Default string            `mapstructure:"default"`

	// Device is the name that was selected, empty when the flat top-level
	// host/port were used.
	Device string `mapstructure:"-"`
}

// DeviceNames returns the configured device names in a stable order.
func (c *Config) DeviceNames() []string {
	names := make([]string, 0, len(c.Devices))
	for name := range c.Devices {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// UnknownDeviceError is returned when --device names something the config file
// does not define. It lists what is available, because the usual cause is a
// typo and the usual next question is "so what are they called".
type UnknownDeviceError struct {
	Name      string
	Available []string
}

func (e *UnknownDeviceError) Error() string {
	if len(e.Available) == 0 {
		return fmt.Sprintf("unknown device %q: no devices are defined in the config file", e.Name)
	}
	return fmt.Sprintf("unknown device %q: defined devices are %s",
		e.Name, strings.Join(e.Available, ", "))
}

// ErrNoHost is returned when no host could be determined from flags,
// environment or config file. Failing here is the point: the alternative is a
// connection attempt against an address the user never chose.
var ErrNoHost = errors.New(
	"no C64 Ultimate configured: pass --host, set C64U_HOST, " +
		"or add a device to ~/.config/c64u/config.toml (run: c64u cli-config init)")

// ExplicitHost and ExplicitPort tell this package whether the user actually
// typed --host or --port. Viper cannot answer that: a bound flag with a default
// always reports as set, so asking it would make every invocation look like an
// explicit override and no device would ever apply its own address. The command
// layer sets these from pflag's Changed state.
var (
	ExplicitHost bool
	ExplicitPort bool
)

// Load loads configuration from file, environment variables, and flags
// Priority: CLI flags > Environment variables > Config file > Defaults
func Load() (*Config, error) {
	debug.Log("Setting default configuration values")
	// Set default values.
	//
	// There is deliberately no default host. A C64 Ultimate is always a
	// separate machine on the network, so "localhost" can only ever be wrong;
	// defaulting to it turned "you have not configured a device" into a
	// connection attempt against this computer and a misleading timeout.
	viper.SetDefault("port", 80)
	viper.SetDefault("verbose", false)
	viper.SetDefault("json", false)

	// Unmarshal only fills keys viper knows about, and AutomaticEnv alone does
	// not register them. "host" has no default any more, so it has to be bound
	// explicitly - otherwise C64U_HOST would be read by Get but silently
	// dropped on the way into the struct.
	_ = viper.BindEnv("host", "C64U_HOST")
	_ = viper.BindEnv("device", "C64U_DEVICE")

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

	if err := cfg.resolveDevice(); err != nil {
		return nil, err
	}

	if cfg.Host == "" {
		return nil, ErrNoHost
	}

	debug.Log("Final config values: device=%q host=%s, port=%d, verbose=%v, json=%v",
		cfg.Device, cfg.Host, cfg.Port, cfg.Verbose, cfg.JSON)

	return &cfg, nil
}

// resolveDevice picks which configured device this invocation talks to and
// copies its host and port into the top-level fields.
//
// Order, first match wins:
//
//  1. --host / --port given explicitly, which override a device as well
//  2. C64U_HOST / C64U_PORT
//  3. --device NAME, or C64U_DEVICE
//  4. the "default" key in the config file
//  5. the only device, when exactly one is defined
//  6. the flat top-level host/port, so existing config files keep working
//  7. the built-in localhost:80
//
// Steps 1 and 2 are already in cfg.Host/cfg.Port by the time this runs: viper
// has applied flags and environment over the file.
func (c *Config) resolveDevice() error {
	name := viper.GetString("device")
	if name == "" {
		name = os.Getenv("C64U_DEVICE")
	}

	explicitHost := ExplicitHost || os.Getenv("C64U_HOST") != ""
	explicitPort := ExplicitPort || os.Getenv("C64U_PORT") != ""

	if name == "" {
		switch {
		case c.Default != "":
			name = c.Default
		case len(c.Devices) == 1:
			name = c.DeviceNames()[0]
		default:
			return nil // flat host/port, or the defaults
		}
	}

	device, ok := c.Devices[name]
	if !ok {
		return &UnknownDeviceError{Name: name, Available: c.DeviceNames()}
	}

	c.Device = name
	// An explicit --host or C64U_HOST outranks the device it would otherwise
	// have taken, so that a one-off address needs no config change.
	if !explicitHost && device.Host != "" {
		c.Host = device.Host
	}
	if device.Port != 0 && !explicitPort {
		c.Port = device.Port
	}
	return nil
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
#
# Fill in the address of your C64 Ultimate below. There is no useful default:
# the device is always a separate machine on the network, so until this is set
# c64u stops with an explanation rather than guessing.

# C64 Ultimate hostname or IP address
# host = "192.168.1.100"

# HTTP port (default: 80)
port = 80

# A second "host" line does not add a device - it replaces the one above.
# For several machines use named entries instead, selected per command with
# --device NAME (or -D NAME):
#
#   c64u --device attic info
#
# "default" decides which one is used when --device is left out. With exactly
# one device defined, it is used and "default" can be left out too.
#
# default = "living-room"
#
# [devices.living-room]
# host = "192.168.1.100"
# port = 80
#
# [devices.attic]
# host = "c64u-attic.local"
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
