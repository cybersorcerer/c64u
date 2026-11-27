package main

import (
	"errors"
	"fmt"
	"time"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/api"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/debug"
	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/wol"
	"github.com/spf13/cobra"
)

const (
	wakeCategory = "U64 Specific Settings"
	wakeItem     = "Wake On Wi-Fi"

	// How long poweron waits for the device to answer; it took 2 s on
	// firmware 1.2.1.
	powerOnLimit    = 60 * time.Second
	powerOnInterval = 2 * time.Second
)

var machinePowerOnCmd = &cobra.Command{
	Use:   "poweron",
	Args:  cobra.NoArgs,
	Short: "Switch the machine on over the network (Wake-on-LAN)",
	Long: `Switch on a C64 Ultimate that was switched off with 'c64u machine poweroff',
by broadcasting a Wake-on-LAN magic packet for its Wi-Fi MAC address.

The device wakes only over Wi-Fi, and only with Wake On Wi-Fi enabled and saved
to flash - it is off by default, and an unsaved setting is gone after power-off:

  c64u config set "U64 Specific Settings" "Wake On Wi-Fi" Enabled
  c64u config save-to-flash "U64 Specific Settings"

A device that is off cannot report its MAC, so it comes from, first match wins:
  --mac                     on the command line
  wifi_mac = "..."          in the device entry of config.toml
  the last poweroff         'c64u machine poweroff' remembers it

The command then waits until the device answers, up to a minute.

Examples:
  c64u machine poweron
  c64u machine poweron --mac 9C:13:9E:ED:D5:EC --no-wait`,
	Run: func(cmd *cobra.Command, args []string) {
		macFlag, _ := cmd.Flags().GetString("mac")
		noWait, _ := cmd.Flags().GetBool("no-wait")

		probe := quickClient()
		if isOn(probe) {
			formatter.Success("Machine is already on", nil)
			return
		}

		mac, source, err := wol.ChooseMAC(macFlag, wifiMAC, wol.Cached(wol.CachePath(), host))
		if err != nil {
			details := []string{err.Error()}
			if errors.Is(err, wol.ErrNoMAC) {
				details = append(details, "Pass --mac, set wifi_mac in the device entry of config.toml, "+
					"or switch the machine off with 'c64u machine poweroff', which remembers the address")
			}
			formatter.Error("Cannot wake the machine", details)
			return
		}
		if err := wol.Send(mac, wol.Destinations); err != nil {
			formatter.Error("Failed to send the wake-up packet", []string{err.Error()})
			return
		}
		details := map[string]interface{}{"mac": mac.String(), "mac_from": source}
		if noWait {
			formatter.Success("Wake-up packet sent", details)
			return
		}

		took, ok := waitUntilOn(func() bool { return isOn(probe) }, powerOnLimit, powerOnInterval, time.Sleep)
		if !ok {
			formatter.Error(fmt.Sprintf("Machine did not answer within %v", powerOnLimit), []string{
				"Wake-up packet sent to " + mac.String() + " (from " + source + ")",
				"It wakes only over Wi-Fi, with " + wakeCategory + " > " + wakeItem + " enabled and saved to flash"})
			return
		}
		formatter.Success(fmt.Sprintf("Machine is on after %v", took), details)
	},
}

// quickClient talks to the device with a short timeout, so asking a device
// that is off does not hang.
func quickClient() *api.Client {
	c := api.NewClient(host, port, false)
	c.Password = password
	c.HTTPClient.Timeout = 3 * time.Second
	return c
}

func isOn(c *api.Client) bool {
	resp, err := c.GetInfo()
	return err == nil && !resp.HasErrors()
}

// waitUntilOn checks every interval until on reports true or limit has passed,
// and returns how long it waited.
func waitUntilOn(on func() bool, limit, interval time.Duration, sleep func(time.Duration)) (time.Duration, bool) {
	for waited := time.Duration(0); waited < limit; {
		sleep(interval)
		waited += interval
		if on() {
			return waited, true
		}
	}
	return limit, false
}

// rememberWifiMAC reads the Wi-Fi MAC and the Wake On Wi-Fi setting before
// poweroff, caches the MAC for poweron, and returns what stands in the way of
// waking the machine again.
func rememberWifiMAC() []string {
	resp, err := apiClient.GetInfo()
	if err != nil || resp.HasErrors() {
		debug.LogError("poweroff: cannot read /v1/info: %v", err)
		return nil
	}
	mac, _ := resp.Data["wifi_mac"].(string)
	if parsed, err := wol.ParseMAC(mac); err == nil {
		if err := wol.Remember(wol.CachePath(), host, parsed); err != nil {
			debug.LogError("poweroff: cannot cache the Wi-Fi MAC: %v", err)
		}
	} else {
		mac = ""
	}
	wake := ""
	if raw, err := apiClient.GetConfigItem(wakeCategory, wakeItem); err == nil {
		if cat, ok := raw[wakeCategory].(map[string]interface{}); ok {
			if detail, ok := cat[wakeItem].(map[string]interface{}); ok {
				wake, _ = detail["current"].(string)
			}
		}
	}
	return wakeWarnings(mac, wake)
}

// wakeWarnings says why machine poweron could not wake the device; nothing
// when it can, or when the firmware has no Wake On Wi-Fi setting to check.
func wakeWarnings(wifiMAC, wakeOnWifi string) []string {
	var warnings []string
	if wifiMAC == "" {
		warnings = append(warnings, "The device reports no Wi-Fi MAC address, so 'c64u machine poweron' "+
			"cannot wake it: Wake-on-LAN works over Wi-Fi only")
	}
	if wakeOnWifi == "Disabled" {
		warnings = append(warnings, wakeItem+" is disabled, so 'c64u machine poweron' cannot wake it. "+
			`Enable it with: c64u config set "`+wakeCategory+`" "`+wakeItem+`" Enabled && `+
			`c64u config save-to-flash "`+wakeCategory+`"`)
	}
	return warnings
}

func init() {
	machineCmd.AddCommand(machinePowerOnCmd)
	machinePowerOnCmd.Flags().String("mac", "", "Wi-Fi MAC address to wake, e.g. 9C:13:9E:ED:D5:EC")
	machinePowerOnCmd.Flags().Bool("no-wait", false, "Send the wake-up packet and return at once")
}
