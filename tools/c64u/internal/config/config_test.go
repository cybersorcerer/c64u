package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func resetViper() {
	viper.Reset()
}

func TestLoad_Defaults(t *testing.T) {
	resetViper()
	// Use an isolated HOME so Load() finds no config file
	t.Setenv("HOME", t.TempDir())
	t.Setenv("C64U_HOST", "")
	t.Setenv("C64U_PORT", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 80 {
		t.Errorf("Port = %d, want 80", cfg.Port)
	}
	if cfg.Verbose {
		t.Error("Verbose should default to false")
	}
	if cfg.JSON {
		t.Error("JSON should default to false")
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	resetViper()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("C64U_HOST", "192.168.1.99")
	t.Setenv("C64U_PORT", "8080")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "192.168.1.99" {
		t.Errorf("Host = %q, want %q", cfg.Host, "192.168.1.99")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want 8080", cfg.Port)
	}
}

func TestLoad_FromFile(t *testing.T) {
	resetViper()

	// Write config into isolated HOME
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)
	t.Setenv("C64U_HOST", "")
	t.Setenv("C64U_PORT", "")

	cfgDir := filepath.Join(tmpHome, ".config", "c64u")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `host = "10.0.0.1"` + "\n" + `port = 9090` + "\n"
	if err := os.WriteFile(filepath.Join(cfgDir, "config.toml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test config: %v", err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if cfg.Host != "10.0.0.1" {
		t.Errorf("Host = %q, want %q", cfg.Host, "10.0.0.1")
	}
	if cfg.Port != 9090 {
		t.Errorf("Port = %d, want 9090", cfg.Port)
	}
}

func TestGetConfigPath_ContainsCofigToml(t *testing.T) {
	path := GetConfigPath()
	if path == "" {
		t.Fatal("GetConfigPath returned empty string")
	}
	if filepath.Base(path) != "config.toml" {
		t.Errorf("config filename = %q, want config.toml", filepath.Base(path))
	}
}

func TestCreateDefaultConfig_CreatesFile(t *testing.T) {
	resetViper()

	// Override home dir via temp dir trick using t.TempDir + HOME env
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	if err := CreateDefaultConfig(); err != nil {
		t.Fatalf("CreateDefaultConfig() error: %v", err)
	}

	expected := filepath.Join(tmpHome, ".config", "c64u", "config.toml")
	if _, err := os.Stat(expected); os.IsNotExist(err) {
		t.Errorf("config file not created at %s", expected)
	}
}

func TestCreateDefaultConfig_FailsIfExists(t *testing.T) {
	resetViper()

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create it once
	if err := CreateDefaultConfig(); err != nil {
		t.Fatalf("first CreateDefaultConfig() error: %v", err)
	}

	// Second call should fail
	if err := CreateDefaultConfig(); err == nil {
		t.Error("expected error on second CreateDefaultConfig(), got nil")
	}
}
