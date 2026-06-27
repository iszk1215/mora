package coverage

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func processDebugOption(cmd *cobra.Command) {
	debug, err := cmd.Flags().GetBool("debug")
	if err != nil {
		return
	}
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
		log.Debug().Msg("debug mode enabled")
	}
}

func newUploadCommand() *cobra.Command {

	var uploadCmd = &cobra.Command{
		Use:   "upload",
		Short: "Upload coverage",

		RunE: func(cmd *cobra.Command, args []string) error {
			processDebugOption(cmd)

			server, err := cmd.Flags().GetString("server")
			if err != nil {
				return fmt.Errorf("failed to get server flag: %w", err)
			}
			repoURL, err := cmd.Flags().GetString("repo")
			if err != nil {
				return fmt.Errorf("failed to get repo flag: %w", err)
			}
			repoPath, err := cmd.Flags().GetString("repo-path")
			if err != nil {
				return fmt.Errorf("failed to get repo-path flag: %w", err)
			}
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return fmt.Errorf("failed to get force flag: %w", err)
			}
			entryName, err := cmd.Flags().GetString("entry")
			if err != nil {
				return fmt.Errorf("failed to get entry flag: %w", err)
			}
			dryRun, err := cmd.Flags().GetBool("dry-run")
			if err != nil {
				return fmt.Errorf("failed to get dry-run flag: %w", err)
			}
			yes, err := cmd.Flags().GetBool("yes")
			if err != nil {
				return fmt.Errorf("failed to get yes flag: %w", err)
			}

			cmd.SilenceUsage = true
			return Upload(server, repoURL, repoPath, entryName, dryRun, force, yes, args)
		},
	}

	uploadCmd.Flags().String("server", "", "server url")
	uploadCmd.Flags().String("repo-path", "", "path of repositry")
	uploadCmd.Flags().String("repo", "", "URL")
	uploadCmd.Flags().String("entry", "_default", "entry name")
	// uploadCmd.Flags().String("revision", "", "revision")
	uploadCmd.Flags().BoolP("force", "f", false, "force upload even when working tree is dirty")
	uploadCmd.Flags().Bool("dry-run", false, "test")
	uploadCmd.Flags().BoolP("yes", "y", false, "yes")

	return uploadCmd
}

func NewCommand() *cobra.Command {
	var cmd = &cobra.Command{
		Use:   "coverage",
		Short: "coverage command",
	}

	cmd.AddCommand(newUploadCommand())

	return cmd

}
