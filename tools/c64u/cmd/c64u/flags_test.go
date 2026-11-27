package main

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// The root's pre-run reads cmd.Flags().Changed("host") and friends, which
// sees a subcommand's own flag of the same name too: a local --host or --port
// then silently replaces the device address. No subcommand may shadow a
// global flag.
func TestNoLocalFlagShadowsAGlobalOne(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		c.LocalNonPersistentFlags().VisitAll(func(f *pflag.Flag) {
			if rootCmd.PersistentFlags().Lookup(f.Name) != nil {
				t.Errorf("%q has its own --%s, which shadows the global flag", c.CommandPath(), f.Name)
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}
