package main

import (
	"fmt"
	"os"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/config"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
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
	cfgFile   string
	host      string
	device    string
	port      int
	verbose   bool
	jsonOut   bool
	noColor   bool
	debugMode bool

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

Several C64 Ultimates can be described in the config file, each under a name,
and --device picks which one a command talks to:

  c64u --device attic info
  c64u -D attic info

Without --device the "default" entry is used, or the only device when just one
is defined. "c64u cli-config show" lists them.

Configuration Priority:
  1. CLI flags (--host, --port)
  2. Environment variables (C64U_HOST, C64U_PORT, C64U_DEVICE)
  3. --device, naming an entry in the config file
  4. Config file (~/.config/c64u/config.toml)
  5. Port defaults to 80. The host has no default - without one, c64u stops
     and says so, because the device is never this machine.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize debug logger first (before anything else)
		if err := debug.Init(debugMode); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize debug logging: %v\n", err)
		}

		debug.Log("Command: %s %v", cmd.Name(), args)
		debug.Log("Loading configuration...")

		// Viper cannot tell a typed flag from its default, so pass on what the
		// user actually gave; without this a device would never override the
		// host that the flag's default already put in place.
		config.ExplicitHost = cmd.Root().PersistentFlags().Changed("host")
		config.ExplicitPort = cmd.Root().PersistentFlags().Changed("port")

		// Initialize configuration
		cfg, err := config.Load()
		if err != nil {
			debug.LogError("Failed to load config: %v", err)
			fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
			os.Exit(1)
		}

		debug.Log("Config loaded successfully: host=%s, port=%d, verbose=%v", cfg.Host, cfg.Port, cfg.Verbose)

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
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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
	Args:  cobra.NoArgs,
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

// cliConfigCmd represents the CLI config command group
var cliConfigCmd = &cobra.Command{
	Use:   "cli-config",
	Args:  cobra.NoArgs,
	Short: "Manage c64u CLI configuration",
	Long:  `View and manage the c64u CLI configuration file (not C64 Ultimate hardware config).`,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help() //nolint:errcheck
	},
}

// configInitCmd creates a default config file
var configInitCmd = &cobra.Command{
	Use:   "init",
	Args:  cobra.NoArgs,
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

// cliConfigShowCmd shows the current CLI configuration
var cliConfigShowCmd = &cobra.Command{
	Use:   "show",
	Args:  cobra.NoArgs,
	Short: "Show current CLI configuration",
	Long:  `Display the current c64u CLI configuration settings being used.`,
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
		if cfg.Device != "" {
			data["device"] = cfg.Device
		}
		if len(cfg.Devices) > 0 {
			devices := make(map[string]interface{}, len(cfg.Devices))
			for name, d := range cfg.Devices {
				devices[name] = map[string]interface{}{"host": d.Host, "port": d.Port}
			}
			data["devices"] = devices
			data["default"] = cfg.Default
		}

		configPath := config.GetConfigPath()
		if configPath != "" {
			data["config_file"] = configPath
		}

		if jsonOut {
			formatter.PrintData(data)
		} else {
			fmt.Println("Current Configuration:")
			if cfg.Device != "" {
				fmt.Printf("  Device:      %s\n", cfg.Device)
			}
			fmt.Printf("  Host:        %s\n", cfg.Host)
			fmt.Printf("  Port:        %d\n", cfg.Port)
			fmt.Printf("  Verbose:     %v\n", cfg.Verbose)
			if configPath != "" {
				fmt.Printf("  Config File: %s\n", configPath)
			}

			if len(cfg.Devices) > 0 {
				fmt.Println()
				fmt.Println("Defined devices:")
				for _, name := range cfg.DeviceNames() {
					d := cfg.Devices[name]
					marker := " "
					if name == cfg.Device {
						marker = "*"
					}
					port := d.Port
					if port == 0 {
						port = 80
					}
					fmt.Printf("  %s %-12s %s:%d\n", marker, name, d.Host, port)
				}
				fmt.Println()
				fmt.Println("  * = used by this invocation. Select with --device NAME.")
			}
		}
	},
}

// isZeroDefault reports whether a flag's default is the zero value of its type,
// which pflag leaves out of the help. pflag decides this with an unexported
// helper, so the zero values are listed here instead.
func isZeroDefault(f *pflag.Flag) bool {
	switch f.DefValue {
	case "", "0", "false", "[]", "<nil>", "0s":
		return true
	}
	return false
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

	// printFlags renders a flag set the way pflag does — name, value type and
	// default — because dropping those left the colored help less informative
	// than the plain one behind --no-color: nothing said that --length is an
	// int defaulting to 256, or that --port defaults to 80.
	printFlags := func(fs *pflag.FlagSet) {
		type row struct{ name, usage string }
		var rows []row
		width := 0

		fs.VisitAll(func(f *pflag.Flag) {
			if f.Hidden {
				return
			}
			varName, usage := pflag.UnquoteUsage(f)

			name := fmt.Sprintf("      --%s", f.Name)
			if f.Shorthand != "" {
				name = fmt.Sprintf("  -%s, --%s", f.Shorthand, f.Name)
			}
			if varName != "" {
				name += " " + varName
			}

			if !isZeroDefault(f) {
				if f.Value.Type() == "string" {
					usage += fmt.Sprintf(" (default %q)", f.DefValue)
				} else {
					usage += fmt.Sprintf(" (default %s)", f.DefValue)
				}
			}

			if len(name) > width {
				width = len(name)
			}
			rows = append(rows, row{name, usage})
		})

		for _, r := range rows {
			fmt.Printf("%s  %s\n", flagStyle.Render(fmt.Sprintf("%-*s", width, r.name)), r.usage)
		}
	}

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

		fmt.Println()
		fmt.Println(sectionStyle.Render("Usage:"))
		fmt.Printf("  %s\n", cmd.UseLine())

		if cmd.HasAvailableSubCommands() {
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

		if cmd.HasAvailableLocalFlags() {
			fmt.Println()
			fmt.Println(sectionStyle.Render("Flags:"))
			printFlags(cmd.LocalFlags())
		}

		if cmd.HasAvailableInheritedFlags() {
			fmt.Println()
			fmt.Println(sectionStyle.Render("Global Flags:"))
			printFlags(cmd.InheritedFlags())
		}

		fmt.Println()
		if cmd.HasAvailableSubCommands() {
			fmt.Printf("Use \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath())
		}
	})
}

func init() {
	// Set up colored help template
	setupColoredHelp()

	// Global flags
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "C64 Ultimate hostname or IP address")
	rootCmd.PersistentFlags().StringVarP(&device, "device", "D", "", "Name of a device defined in the config file")
	rootCmd.PersistentFlags().IntVar(&port, "port", 80, "HTTP port")
	rootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output in JSON format")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().BoolVarP(&debugMode, "debug", "d", false, "Enable debug logging to ~/.local/share/c64u/c64u.log")

	// Bind flags to viper
	viper.BindPFlag("host", rootCmd.PersistentFlags().Lookup("host"))
	viper.BindPFlag("device", rootCmd.PersistentFlags().Lookup("device"))
	viper.BindPFlag("port", rootCmd.PersistentFlags().Lookup("port"))
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("json", rootCmd.PersistentFlags().Lookup("json"))
	viper.BindPFlag("no-color", rootCmd.PersistentFlags().Lookup("no-color"))

	// Add commands
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(aboutCmd)
	rootCmd.AddCommand(infoCmd)
	rootCmd.AddCommand(cliConfigCmd)
	rootCmd.AddCommand(configCmd)
	rootCmd.AddCommand(runnersCmd)
	rootCmd.AddCommand(machineCmd)
	rootCmd.AddCommand(drivesCmd)
	rootCmd.AddCommand(streamsCmd)
	rootCmd.AddCommand(filesCmd)
	rootCmd.AddCommand(fsCmd)

	// CLI Config subcommands
	cliConfigCmd.AddCommand(configInitCmd)
	cliConfigCmd.AddCommand(cliConfigShowCmd)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
