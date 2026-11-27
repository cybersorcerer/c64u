package main

import (
	"fmt"
	"os"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/spf13/cobra"
)

var configRestoreCmd = &cobra.Command{
	Use:   "restore <file>",
	Short: "Put a configuration backup back onto the device",
	Long: `Restore a backup made with 'config export' onto the same device, completely
and permanently.

Every setting in the file that differs from the device is changed, including the
network settings, host name and unique id, which come last. Afterwards the
configuration is saved to flash, so it survives a reboot.

The file may also be in the flat format 'config set-multiple' reads. Settings
this device does not know (a backup from other firmware) are reported and
skipped; the rest is applied. --dry-run lists what would change and writes
nothing.

To copy settings onto a different device, use 'config import' instead: restore
would give it this device's address and name.

Examples:
  c64u config export backup.json             # make the backup
  c64u config restore backup.json --dry-run  # see what would change
  c64u config restore backup.json            # put it back`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		applyConfigFile(args[0], true, dryRun)
	},
}

var configImportCmd = &cobra.Command{
	Use:   "import <file>",
	Short: "Copy settings from a file onto this device, keeping its identity",
	Long: `Copy the settings in a file - typically another device's 'config export' -
onto this device.

Everything is copied except what makes a device unique: the Ethernet and Wi-Fi
settings (DHCP, static address, gateway, DNS), the host name and the unique id.
The rest of the network settings - time zone, time servers, services, password -
is copied.

The changes take effect at once but live in memory only, like 'config set':
'config save-to-flash' keeps them, 'config load-from-flash' undoes them. Settings
this device does not know are reported and skipped. --dry-run lists what would
change and writes nothing.

To put a backup back onto the device it came from, use 'config restore'.

Examples:
  c64u config export my-setup.json                                # on the first device
  c64u --host c64u-two.local config import my-setup.json --dry-run
  c64u --host c64u-two.local config import my-setup.json
  c64u --host c64u-two.local config save-to-flash                 # keep it`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		applyConfigFile(args[0], false, dryRun)
	},
}

// applyConfigFile changes the settings in path that differ from the device.
// restore also applies the identity settings and saves to flash.
func applyConfigFile(path string, restore, dryRun bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		formatter.Error("Failed to read file", []string{err.Error()})
		return
	}
	wanted, err := api.ParseConfigFile(data)
	if err != nil {
		formatter.Error("Invalid configuration file", []string{err.Error()})
		return
	}
	current, err := readDeviceConfig()
	if err != nil {
		formatter.Error("Failed to read the device configuration", []string{err.Error()})
		return
	}

	all := api.DiffConfig(current, wanted)
	unchanged := countSettings(wanted) - len(all)
	rest, identity := splitIdentity(all)
	rest, password := splitPassword(rest)
	if !restore {
		identity = nil
	}
	changes := append(append(append([]api.ConfigChange{}, rest...), identity...), password...)

	if dryRun {
		formatter.Success(fmt.Sprintf("%d setting(s) would change, nothing written", len(changes)), changeReport(changes, nil, unchanged))
		return
	}

	// Reihenfolge: Rest, dann Identität (kann die Adresse ändern), dann das
	// Passwort (wirkt sofort, jede spätere Anfrage braucht das neue), dann Flash.
	failed, err := apiClient.ApplyConfig(rest)
	if err != nil {
		formatter.Error("Failed to apply the settings", []string{err.Error()})
		return
	}
	if len(identity) > 0 {
		f, err := apiClient.ApplyConfig(identity)
		if err != nil {
			formatter.Error("The device did not confirm the network settings", networkHints(err, wanted, identity))
			return
		}
		failed = append(failed, f...)
	}
	passwordChanged := false
	if len(password) > 0 {
		f, err := apiClient.ApplyConfig(password)
		if err != nil {
			formatter.Error("Failed to change the network password", []string{err.Error()})
			return
		}
		if len(f) == 0 {
			apiClient.Password = fmt.Sprint(password[0].New)
			passwordChanged = true
		}
		failed = append(failed, f...)
	}
	if restore {
		if err := apiClient.SaveConfigToFlash(); err != nil {
			hints := []string{err.Error(), "Save them with: c64u config save-to-flash"}
			if len(identity) > 0 {
				hints = networkHints(err, wanted, identity)
			}
			if passwordChanged {
				hints = append(hints, "The network password changed: pass the new one with --password.")
			}
			formatter.Error("Settings applied but not saved to flash", hints)
			return
		}
	}

	title := fmt.Sprintf("%d setting(s) changed", len(changes))
	if len(failed) > 0 {
		title = fmt.Sprintf("%d setting(s) sent, the device rejected some (see failed)", len(changes))
	}
	switch {
	case restore:
		title += "; saved to flash"
	case len(changes) > 0:
		title += " in memory; keep them with: c64u config save-to-flash"
	}
	report := changeReport(changes, failed, unchanged)
	if passwordChanged {
		report["note"] = "The network password changed: pass the new one with --password or C64U_PASSWORD from now on."
	}
	formatter.Success(title, report)
}

// countSettings counts the settings in a file.
func countSettings(settings api.ConfigSettings) int {
	n := 0
	for _, items := range settings {
		n += len(items)
	}
	return n
}

// readDeviceConfig reads every category the way config export does.
func readDeviceConfig() (api.ConfigSettings, error) {
	categories, err := apiClient.GetConfigCategories()
	if err != nil {
		return nil, err
	}
	settings := api.ConfigSettings{}
	for _, cat := range categories {
		resp, err := apiClient.GetConfigCategory(cat)
		if err != nil {
			return nil, fmt.Errorf("category %q: %w", cat, err)
		}
		items, _ := resp[cat].(map[string]interface{})
		settings[cat] = items
	}
	return settings, nil
}

// splitIdentity separates the device identity settings from the rest.
func splitIdentity(changes []api.ConfigChange) (rest, identity []api.ConfigChange) {
	for _, ch := range changes {
		if api.IsIdentity(ch.Category, ch.Item) {
			identity = append(identity, ch)
		} else {
			rest = append(rest, ch)
		}
	}
	return rest, identity
}

// isPassword reports the setting that guards the REST API itself.
func isPassword(ch api.ConfigChange) bool {
	return ch.Category == "Network Settings" && ch.Item == "Network Password"
}

// splitPassword takes the network password out: it takes effect at once, so
// it has to be the last change sent.
func splitPassword(changes []api.ConfigChange) (rest, password []api.ConfigChange) {
	for _, ch := range changes {
		if isPassword(ch) {
			password = append(password, ch)
		} else {
			rest = append(rest, ch)
		}
	}
	return rest, password
}

// staticAddress is the address the device takes on from settings, if they
// give it a static one on Ethernet or Wi-Fi.
func staticAddress(settings api.ConfigSettings) string {
	for _, cat := range []string{"Ethernet Settings", "WiFi settings"} {
		s := settings[cat]
		if fmt.Sprint(s["Use DHCP"]) == "Disabled" && s["Static IP"] != nil {
			return fmt.Sprint(s["Static IP"])
		}
	}
	return ""
}

// networkHints explains a failure after the identity settings were sent. Only
// Ethernet and Wi-Fi settings can move the device; a new host name cannot.
func networkHints(err error, wanted api.ConfigSettings, identity []api.ConfigChange) []string {
	hints := []string{err.Error()}
	moved := false
	for _, ch := range identity {
		if ch.Category == "Ethernet Settings" || ch.Category == "WiFi settings" {
			moved = true
		}
	}
	if !moved {
		return append(hints, "The address did not change. Finish with: c64u config save-to-flash")
	}
	hints = append(hints, "The device may now answer at another address.")
	if addr := staticAddress(wanted); addr != "" {
		return append(hints, "Finish with: c64u --host "+addr+" config save-to-flash")
	}
	return append(hints, "Find its new DHCP address, then finish with: c64u --host <address> config save-to-flash")
}

// changeValue shows a change as "old -> new"; the password stays hidden.
func changeValue(ch api.ConfigChange) string {
	if isPassword(ch) {
		return "(hidden) -> (hidden)"
	}
	old := "(none)"
	switch {
	case ch.Missing:
		old = "(not on this device)"
	case ch.Old != nil:
		old = fmt.Sprint(ch.Old)
	}
	return fmt.Sprintf("%s -> %v", old, ch.New)
}

func changeReport(changes []api.ConfigChange, failed []string, unchanged int) map[string]interface{} {
	report := map[string]interface{}{"unchanged": unchanged}
	if jsonOut {
		list := make([]map[string]interface{}, len(changes))
		for i, ch := range changes {
			old, nw := ch.Old, ch.New
			if isPassword(ch) {
				old, nw = "(hidden)", "(hidden)"
			}
			list[i] = map[string]interface{}{"category": ch.Category, "item": ch.Item, "old": old, "new": nw, "missing": ch.Missing}
		}
		report["changes"] = list
	} else {
		// Nach Kategorie verschachtelt, damit der Formatter eingerückt ausgibt
		byCat := map[string]interface{}{}
		for _, ch := range changes {
			items, _ := byCat[ch.Category].(map[string]interface{})
			if items == nil {
				items = map[string]interface{}{}
				byCat[ch.Category] = items
			}
			items[ch.Item] = changeValue(ch)
		}
		report["changes"] = byCat
	}
	if len(failed) > 0 {
		report["failed"] = failed
	}
	return report
}

func init() {
	for _, c := range []*cobra.Command{configRestoreCmd, configImportCmd} {
		c.Flags().Bool("dry-run", false, "List what would change and write nothing")
		configCmd.AddCommand(c)
	}
}
