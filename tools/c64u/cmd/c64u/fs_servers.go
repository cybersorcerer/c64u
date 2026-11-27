package main

import (
	"fmt"

	"github.com/cybersorcerer/c64.nvim/tools/c64u/internal/remoteftp"
	"github.com/spf13/cobra"
)

// serversTakeEffect is said wherever the server list changes.
const serversTakeEffect = "The device reads the list when it is switched on: " +
	"c64u machine poweroff, then c64u machine poweron. A server added in the Ultimate menu " +
	"(Disk File Browser > FTP > New Host) works at once"

var fsServersCmd = &cobra.Command{
	Use:   "servers",
	Args:  cobra.NoArgs,
	Short: "List the remote FTP servers the device mounts under /ftp",
	Long: `List the remote FTP servers - a NAS, another computer - that the C64 Ultimate
mounts under /ftp/<alias>/. Their files can then be used like the device's own:

  c64u drives mount a /ftp/nas/games/disk.d64
  c64u runners run-prg /ftp/nas/demo.prg

The list is kept in ` + remoteftp.ServerFile + `, which the device reads when it is
switched on. 'add' and 'remove' change that file; a server added in the Ultimate
menu (Disk File Browser > FTP > New Host) works at once and lands in the same file.

The password is stored on the device URL-encoded, which is not encryption:
anyone with FTP access to the C64 Ultimate can read it. Give the device an
account that may only read.

Examples:
  c64u fs servers
  c64u fs servers add nas 192.168.1.20 --user c64 --server-password secret --folder /c64
  c64u fs servers remove nas`,
	Run: func(cmd *cobra.Command, args []string) {
		servers, err := remoteftp.Load(apiClient)
		if err != nil {
			formatter.Error("Failed to read the remote FTP servers", []string{err.Error()})
			return
		}
		if len(servers) == 0 {
			formatter.Success("No remote FTP servers", nil)
			return
		}
		rows := []interface{}{}
		for _, s := range servers {
			password := "none"
			if s.Password != "" {
				password = "set"
			}
			rows = append(rows, map[string]interface{}{
				"alias": s.Alias, "host": s.Host, "port": s.Port, "user": s.User,
				"folder": s.Folder, "password": password, "path": "/ftp/" + s.Alias,
			})
		}
		formatter.Success("Remote FTP servers", map[string]interface{}{"servers": rows})
	},
}

var fsServersAddCmd = &cobra.Command{
	Use:   "add <alias> <host>",
	Short: "Add or change a remote FTP server",
	Long: `Add a remote FTP server, or replace the one with that alias - every setting
not given falls back to its default. The alias becomes the directory
/ftp/<alias>/ on the device.

The password is stored readable on the device; see 'c64u fs servers --help'.
(--password and --port belong to the C64 Ultimate itself, so the server's are
--server-password and --server-port.)

` + serversTakeEffect + `.

Examples:
  c64u fs servers add nas 192.168.1.20
  c64u fs servers add nas nas.local --server-port 2121 --user c64 --server-password secret --folder /c64`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		port, _ := cmd.Flags().GetInt("server-port")
		user, _ := cmd.Flags().GetString("user")
		secret, _ := cmd.Flags().GetString("server-password")
		folder, _ := cmd.Flags().GetString("folder")

		server, err := remoteftp.NewServer(args[0], args[1], port, user, secret, folder)
		if err != nil {
			formatter.Error("Invalid server", []string{err.Error()})
			return
		}
		servers, err := remoteftp.Load(apiClient)
		if err != nil {
			formatter.Error("Failed to read the remote FTP servers", []string{err.Error()})
			return
		}
		servers, replaced := remoteftp.Put(servers, server)
		if err := remoteftp.Save(apiClient, servers); err != nil {
			formatter.Error("Failed to save the remote FTP servers", []string{err.Error()})
			return
		}
		verb := "added"
		if replaced {
			verb = "changed"
		}
		formatter.Success(fmt.Sprintf("Remote FTP server %s %s", server.Alias, verb), map[string]interface{}{
			"path": "/ftp/" + server.Alias,
			"note": serversTakeEffect,
		})
	},
}

var fsServersRemoveCmd = &cobra.Command{
	Use:   "remove <alias>",
	Short: "Remove a remote FTP server",
	Long: `Remove the remote FTP server with that alias from the device's list.

` + serversTakeEffect + `.

Example:
  c64u fs servers remove nas`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		servers, err := remoteftp.Load(apiClient)
		if err != nil {
			formatter.Error("Failed to read the remote FTP servers", []string{err.Error()})
			return
		}
		servers, ok := remoteftp.Remove(servers, args[0])
		if !ok {
			formatter.Error("No such remote FTP server", []string{fmt.Sprintf("%q is not in the list; see c64u fs servers", args[0])})
			return
		}
		if err := remoteftp.Save(apiClient, servers); err != nil {
			formatter.Error("Failed to save the remote FTP servers", []string{err.Error()})
			return
		}
		formatter.Success(fmt.Sprintf("Remote FTP server %s removed", args[0]), map[string]interface{}{"note": serversTakeEffect})
	},
}

func init() {
	fsCmd.AddCommand(fsServersCmd)
	fsServersCmd.AddCommand(fsServersAddCmd)
	fsServersCmd.AddCommand(fsServersRemoveCmd)

	fsServersAddCmd.Flags().Int("server-port", 21, "FTP port of the server")
	fsServersAddCmd.Flags().String("user", "anonymous", "User name")
	fsServersAddCmd.Flags().String("server-password", "", "Password for that user (stored readable on the device)")
	fsServersAddCmd.Flags().String("folder", "/", "Folder on the server that /ftp/<alias>/ shows")
}
