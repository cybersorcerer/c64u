package main

import (
	"fmt"
	"os"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/config"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var (
	// Version information (set by build flags)
	version = "dev"
	commit  = "none"
	date    = "unknown"

	// Global flags
	cfgFile string
	host    string
	port    int
	verbose bool
	jsonOut bool
	noColor bool

	// Global instances
	apiClient *api.Client
	formatter *output.Formatter
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "c64u",
	Short: "CLI tool for controlling the Commodore C64 Ultimate",
	Long: `c64u is a command-line interface for the Commodore C64 Ultimate REST API.

It allows you to control your C64 Ultimate hardware from the command line,
including uploading and running programs, managing disk images, controlling
the machine state, and more.

Configuration Priority:
  1. CLI flags (--host, --port)
  2. Environment variables (C64U_HOST, C64U_PORT)
  3. Config file (~/.config/c64u/config.toml)
  4. Defaults (host=localhost, port=80)`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize configuration
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		// Override with command-line flags if provided
		if cmd.Flags().Changed("host") {
			cfg.Host = host
		} else {
			host = cfg.Host
		}

		if cmd.Flags().Changed("port") {
			cfg.Port = port
		} else {
			port = cfg.Port
		}

		if cmd.Flags().Changed("verbose") {
			cfg.Verbose = verbose
		} else {
			verbose = cfg.Verbose
		}

		if cmd.Flags().Changed("json") {
			cfg.JSON = jsonOut
		} else {
			jsonOut = cfg.JSON
		}

		// Initialize global instances
		apiClient = api.NewClient(cfg.Host, cfg.Port, cfg.Verbose)
		formatter = output.NewFormatter(cfg.JSON)
		formatter.SetNoColor(noColor)
	},
}

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version, build commit, and build date of the c64u CLI tool.`,
	Run: func(cmd *cobra.Command, args []string) {
		if jsonOut {
			data := map[string]interface{}{
				"version": version,
				"commit":  commit,
				"date":    date,
			}
			formatter.PrintData(data)
		} else {
			fmt.Printf("c64u version %s\n", version)
			fmt.Printf("  commit: %s\n", commit)
			fmt.Printf("  built:  %s\n", date)
		}
	},
}

// aboutCmd gets the API version from the C64 Ultimate
var aboutCmd = &cobra.Command{
	Use:   "about",
	Short: "Get C64 Ultimate API version",
	Long:  `Query the C64 Ultimate to retrieve its REST API version (calls /v1/version).`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.Get("/v1/version", nil)
		if err != nil {
			formatter.Error("Failed to get API version", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		if jsonOut {
			formatter.PrintData(resp.Data)
		} else {
			apiVersion := resp.GetString("version")
			if apiVersion != "" {
				fmt.Printf("C64 Ultimate API version: %s\n", apiVersion)
			} else {
				formatter.PrintData(resp.Data)
			}
		}
	},
}

// infoCmd gets device information from the C64 Ultimate
var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Get C64 Ultimate device information",
	Long:  `Query the C64 Ultimate to retrieve device information including product name, firmware versions, and hostname (calls /v1/info).`,
	Run: func(cmd *cobra.Command, args []string) {
		resp, err := apiClient.GetInfo()
		if err != nil {
			formatter.Error("Failed to get device info", []string{err.Error()})
			return
		}

		if resp.HasErrors() {
			formatter.Error("API returned errors", resp.Errors)
			return
		}

		if jsonOut {
			formatter.PrintData(resp.Data)
		} else {
			product := resp.GetString("product")
			firmware := resp.GetString("firmware_version")
			fpga := resp.GetString("fpga_version")
			core := resp.GetString("core_version")
			hostname := resp.GetString("hostname")
			uniqueID := resp.GetString("unique_id")

			formatter.PrintHeader("C64 Ultimate Device Information")
			fmt.Println()
			if product != "" {
				formatter.PrintKeyValue("Product", product)
			}
			if firmware != "" {
				formatter.PrintKeyValue("Firmware Version", firmware)
			}
			if fpga != "" {
				formatter.PrintKeyValue("FPGA Version", fpga)
			}
			if core != "" {
				formatter.PrintKeyValue("Core Version", core)
			}
			if hostname != "" {
				formatter.PrintKeyValue("Hostname", hostname)
			}
			if uniqueID != "" {
				formatter.PrintKeyValue("Unique ID", uniqueID)
			}
		}
	},
}

// configCmd represents the config command group
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage c64u configuration",
	Long:  `View and manage the c64u CLI configuration file.`,
}

// configInitCmd creates a default config file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create default configuration file",
	Long:  `Create a default configuration file at ~/.config/c64u/config.toml`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := config.CreateDefaultConfig(); err != nil {
			formatter.Error("Failed to create config file", []string{err.Error()})
			return
		}

		configPath := config.GetConfigPath()
		formatter.Success("Configuration file created", map[string]interface{}{
			"path": configPath,
		})
	},
}

// configShowCmd shows the current configuration
var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Show current configuration",
	Long:  `Display the current configuration settings being used.`,
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := config.Load()
		if err != nil {
			formatter.Error("Failed to load config", []string{err.Error()})
			return
		}

		data := map[string]interface{}{
			"host":    cfg.Host,
			"port":    cfg.Port,
			"verbose": cfg.Verbose,
		}

		configPath := config.GetConfigPath()
		if configPath != "" {
			data["config_file"] = configPath
		}

		if jsonOut {
			formatter.PrintData(data)
		} else {
			fmt.Println("Current Configuration:")
			fmt.Printf("  Host:        %s\n", cfg.Host)
			fmt.Printf("  Port:        %d\n", cfg.Port)
			fmt.Printf("  Verbose:     %v\n", cfg.Verbose)
			if configPath != "" {
				fmt.Printf("  Config File: %s\n", configPath)
			}
		}
	},
}

// setupColoredHelp configures Cobra to use colored output in help text
func setupColoredHelp() {
	// Import lipgloss for colored help
	titleStyle := output.NewFormatter(false).GetTitleStyle()
	sectionStyle := output.NewFormatter(false).GetSectionStyle()
	commandStyle := output.NewFormatter(false).GetCommandStyle()
	flagStyle := output.NewFormatter(false).GetFlagStyle()

	// Store default help function
	defaultHelpFunc := rootCmd.HelpFunc()

	// Custom help template with colors
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, args []string) {
		// Check if colors should be disabled
		if noColor {
			defaultHelpFunc(cmd, args)
			return
		}

		fmt.Println(titleStyle.Render(cmd.Short))
		if cmd.Long != "" {
			fmt.Println()
			fmt.Println(cmd.Long)
		}

		if cmd.HasAvailableSubCommands() {
			fmt.Println()
			fmt.Println(sectionStyle.Render("Usage:"))
			fmt.Printf("  %s\n", cmd.UseLine())

			fmt.Println()
			fmt.Println(sectionStyle.Render("Available Commands:"))
			for _, c := range cmd.Commands() {
				if !c.IsAvailableCommand() || c.IsAdditionalHelpTopicCommand() {
					continue
				}
				fmt.Printf("  %s  %s\n",
					commandStyle.Render(fmt.Sprintf("%-15s", c.Name())),
					c.Short)
			}
		}

		if cmd.HasAvailableLocalFlags() || cmd.HasAvailableInheritedFlags() {
			fmt.Println()
			fmt.Println(sectionStyle.Render("Flags:"))
			cmd.Flags().VisitAll(func(f *pflag.Flag) {
				if f.Hidden {
					return
				}
				flagName := fmt.Sprintf("  -%s, --%s", f.Shorthand, f.Name)
				if f.Shorthand == "" {
					flagName = fmt.Sprintf("      --%s", f.Name)
				}
				fmt.Printf("%s  %s\n",
					flagStyle.Render(fmt.Sprintf("%-20s", flagName)),
					f.Usage)
			})
		}

		fmt.Println()
		fmt.Printf("Use \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
	})
}

func init() {
	// Set up colored help template
	setupColoredHelp()

	// Global flags
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "C64 Ultimate hostname or IP address")
	rootCmd.PersistentFlags().IntVar(&port, "port", 80, "HTTP port")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")

	// Bind flags to viper
	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	viper.BindPFlag("no-color", rootCmd.PersistentFlags().Lookup("no-color"))

	// Add commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(aboutCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(runnersCmd)
	rootCmd.AddCommand(machineCmd)
	rootCmd.AddCommand(drivesCmd)
	rootCmd.AddCommand(streamsCmd)
	rootCmd.AddCommand(filesCmd)

	// Config subcommands
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
