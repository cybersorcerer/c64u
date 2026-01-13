package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// ============================================================================
// CONFIG COMMANDS (C64 Ultimate Hardware Configuration)
// ============================================================================

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage C64 Ultimate hardware configuration",
	Long: `Manage C64 Ultimate hardware configuration settings.

This manages the C64 Ultimate device settings (drives, machine, etc.),
not the c64u CLI configuration file. For CLI config, use 'cli-config'.`,
}

// ============================================================================
// CONFIG LIST - List all configuration categories
// ============================================================================

var configListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all configuration categories",
	Long: `List all available configuration categories on the C64 Ultimate.

Examples:
  c64u config list`,
	Run: func(cmd *cobra.Command, args []string) {
		categories, err := apiClient.GetConfigCategories()
		if err != nil {
			formatter.Error("Failed to get config categories", []string{err.Error()})
			return
		}

		if jsonOut {
			formatter.PrintData(map[string]interface{}{
				"categories": categories,
			})
		} else {
			formatter.PrintHeader("Configuration Categories")
			fmt.Println()

			for _, cat := range categories {
				fmt.Printf("  • %s\n", cat)
			}
		}
	},
}

// ============================================================================
// CONFIG SHOW - Show all settings in a category
// ============================================================================

var configShowCmd = &cobra.Command{
	Use:   "show <category>",
	Short: "Show all settings in a category",
	Long: `Display all configuration settings in a category.

Category names support wildcards (e.g., "drive a*").

Examples:
  c64u config show "Drive A Settings"
  c64u config show "drive a*"`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		category := args[0]

		settings, err := apiClient.GetConfigCategory(category)
		if err != nil {
			formatter.Error("Failed to get category settings", []string{err.Error()})
			return
		}

		if jsonOut {
			formatter.PrintData(settings)
		} else {
			formatter.PrintHeader(fmt.Sprintf("Category: %s", category))
			fmt.Println()

			for key, value := range settings {
				formatter.PrintKeyValue(key, fmt.Sprintf("%v", value))
			}
		}
	},
}

// ============================================================================
// CONFIG GET - Get detailed info about a config item
// ============================================================================

var configGetCmd = &cobra.Command{
	Use:   "get <category> <item>",
	Short: "Get detailed info about a config item",
	Long: `Get detailed information about a specific configuration item.

Both category and item support wildcards.

Examples:
  c64u config get "Drive A Settings" "Drive Type"
  c64u config get "drive a*" "*bus*"`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		category := args[0]
		item := args[1]

		info, err := apiClient.GetConfigItem(category, item)
		if err != nil {
			formatter.Error("Failed to get config item", []string{err.Error()})
			return
		}

		if jsonOut {
			formatter.PrintData(info)
		} else {
			formatter.PrintHeader(fmt.Sprintf("%s / %s", category, item))
			fmt.Println()

			for key, value := range info {
				formatter.PrintKeyValue(key, fmt.Sprintf("%v", value))
			}
		}
	},
}

// ============================================================================
// CONFIG SET - Set a configuration item
// ============================================================================

var configSetCmd = &cobra.Command{
	Use:   "set <category> <item> <value>",
	Short: "Set a configuration item",
	Long: `Set a configuration item to a new value.

Both category and item support wildcards.

Examples:
  c64u config set "Drive A Settings" "Drive Type" "1581"
  c64u config set "drive a*" "*bus*" "9"`,
	Args: cobra.ExactArgs(3),
	Run: func(cmd *cobra.Command, args []string) {
		category := args[0]
		item := args[1]
		value := args[2]

		if err := apiClient.SetConfigItem(category, item, value); err != nil {
			formatter.Error("Failed to set config item", []string{err.Error()})
			return
		}

		formatter.Success("Configuration updated", map[string]interface{}{
			"category": category,
			"item":     item,
			"value":    value,
		})
	},
}

// ============================================================================
// CONFIG SET-MULTIPLE - Set multiple config items from JSON
// ============================================================================

var configSetMultipleCmd = &cobra.Command{
	Use:   "set-multiple <json-file>",
	Short: "Set multiple config items from JSON file",
	Long: `Set multiple configuration items from a JSON file.

JSON format:
{
  "Drive A Settings": {
    "Drive": "Enabled",
    "Drive Type": "1581"
  },
  "Drive B Settings": {
    "Drive": "Disabled"
  }
}

Examples:
  c64u config set-multiple settings.json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]

		data, err := os.ReadFile(filename)
		if err != nil {
			formatter.Error("Failed to read file", []string{err.Error()})
			return
		}

		var settings map[string]map[string]interface{}
		if err := json.Unmarshal(data, &settings); err != nil {
			formatter.Error("Failed to parse JSON", []string{err.Error()})
			return
		}

		if err := apiClient.SetMultipleConfigs(settings); err != nil {
			formatter.Error("Failed to set config items", []string{err.Error()})
			return
		}

		count := 0
		for _, items := range settings {
			count += len(items)
		}

		formatter.Success("Configuration updated", map[string]interface{}{
			"categories": len(settings),
			"items":      count,
			"file":       filename,
		})
	},
}

// ============================================================================
// CONFIG SAVE-TO-FLASH - Save config to non-volatile memory
// ============================================================================

var configSaveToFlashCmd = &cobra.Command{
	Use:   "save-to-flash",
	Short: "Save configuration to flash memory",
	Long: `Save current configuration to non-volatile flash memory.

This makes the current settings permanent across reboots.

Examples:
  c64u config save-to-flash`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := apiClient.SaveConfigToFlash(); err != nil {
			formatter.Error("Failed to save config to flash", []string{err.Error()})
			return
		}

		formatter.Success("Configuration saved to flash", nil)
	},
}

// ============================================================================
// CONFIG LOAD-FROM-FLASH - Load config from flash memory
// ============================================================================

var configLoadFromFlashCmd = &cobra.Command{
	Use:   "load-from-flash",
	Short: "Load configuration from flash memory",
	Long: `Load configuration from non-volatile flash memory.

WARNING: This discards any unsaved changes to the current configuration.

Examples:
  c64u config load-from-flash`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := apiClient.LoadConfigFromFlash(); err != nil {
			formatter.Error("Failed to load config from flash", []string{err.Error()})
			return
		}

		formatter.Success("Configuration loaded from flash", nil)
	},
}

// ============================================================================
// CONFIG RESET-TO-DEFAULT - Reset to factory defaults
// ============================================================================

var configResetToDefaultCmd = &cobra.Command{
	Use:   "reset-to-default",
	Short: "Reset configuration to factory defaults",
	Long: `Reset current configuration to factory default values.

WARNING: This does not affect saved values in flash memory.
Use save-to-flash after reset to make defaults permanent.

Examples:
  c64u config reset-to-default`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := apiClient.ResetConfigToDefault(); err != nil {
			formatter.Error("Failed to reset config", []string{err.Error()})
			return
		}

		formatter.Success("Configuration reset to defaults", nil)
	},
}

// ============================================================================
// CONFIG EXPORT - Export all settings to JSON
// ============================================================================

var configExportCmd = &cobra.Command{
	Use:   "export [output-file]",
	Short: "Export all configuration to JSON",
	Long: `Export all configuration settings to a JSON file.

If no output file is specified, prints to stdout.

Examples:
  c64u config export config-backup.json
  c64u config export`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Get all categories
		categories, err := apiClient.GetConfigCategories()
		if err != nil {
			formatter.Error("Failed to get config categories", []string{err.Error()})
			return
		}

		// Get all settings for each category
		allSettings := make(map[string]map[string]interface{})
		for _, cat := range categories {
			settings, err := apiClient.GetConfigCategory(cat)
			if err != nil {
				formatter.Error(fmt.Sprintf("Failed to get category '%s'", cat), []string{err.Error()})
				return
			}
			allSettings[cat] = settings
		}

		// Marshal to JSON
		jsonData, err := json.MarshalIndent(allSettings, "", "  ")
		if err != nil {
			formatter.Error("Failed to marshal JSON", []string{err.Error()})
			return
		}

		// Write to file or stdout
		if len(args) > 0 {
			filename := args[0]
			if err := os.WriteFile(filename, jsonData, 0644); err != nil {
				formatter.Error("Failed to write file", []string{err.Error()})
				return
			}
			formatter.Success("Configuration exported", map[string]interface{}{
				"file":       filename,
				"categories": len(allSettings),
			})
		} else {
			fmt.Println(string(jsonData))
		}
	},
}

func init() {
	// Add subcommands to config
	configCmd.AddCommand(configListCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configSetMultipleCmd)
	configCmd.AddCommand(configSaveToFlashCmd)
	configCmd.AddCommand(configLoadFromFlashCmd)
	configCmd.AddCommand(configResetToDefaultCmd)
	configCmd.AddCommand(configExportCmd)
}
