/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"
	"time"

	"github.com/iszk1215/mora/mora/upload"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// uploadCmd represents the upload command
var uploadCmd = &cobra.Command{
	Use:   "upload",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		noColor := false
		o, _ := os.Stderr.Stat()
		if (o.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
			noColor = true
		}

		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: noColor}).With().Caller().Logger()

		server, _ := cmd.Flags().GetString("server")
		repoURL, _ := cmd.Flags().GetString("repo")
		repoPath, _ := cmd.Flags().GetString("repo-path")
		force, _ := cmd.Flags().GetBool("force")
		entryName, _ := cmd.Flags().GetString("entry")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		return upload.Upload(server, repoURL, repoPath, entryName, dryRun, force, args)
	},
}

func init() {
	rootCmd.AddCommand(uploadCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// uploadCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// uploadCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	uploadCmd.Flags().String("server", "", "server url")
	uploadCmd.Flags().String("repo-path", "", "path of repositry")
	uploadCmd.Flags().String("repo", "", "URL")
	uploadCmd.Flags().String("entry", "_default", "entry name")
	uploadCmd.Flags().BoolP("force", "f", false, "force upload even when working tree is dirty")
	uploadCmd.Flags().Bool("dry-run", false, "test")
}
