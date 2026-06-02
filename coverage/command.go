package coverage

import (
	"fmt"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"
)

func processDebugOption(cmd *cobra.Command) {
	debug, _ := cmd.Flags().GetBool("debug")
	fmt.Printf("debug=%t\n", debug)
	if debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}
}

func newUploadCommand() *cobra.Command {

	var uploadCmd = &cobra.Command{
		Use:   "upload",
		Short: "Upload coverage",

		RunE: func(cmd *cobra.Command, args []string) error {
			processDebugOption(cmd)

			server, _ := cmd.Flags().GetString("server")
			repoURL, _ := cmd.Flags().GetString("repo")
			repoPath, _ := cmd.Flags().GetString("repo-path")
			force, _ := cmd.Flags().GetBool("force")
			entryName, _ := cmd.Flags().GetString("entry")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			yes, _ := cmd.Flags().GetBool("yes")

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
