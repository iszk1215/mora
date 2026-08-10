package cmd

import (
	"os"
	"time"

	"github.com/iszk1215/mora/coverage"
	"github.com/iszk1215/mora/udm"
	"github.com/iszk1215/mora/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	noColor := false
	o, _ := os.Stderr.Stat()
	if (o.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		noColor = true
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339, NoColor: noColor}).With().Caller().Logger()

	var cmd = &cobra.Command{
		Use:     "mora",
		Short:   "Mora is a coverage tracker",
		Version: version.Version,
	}

	cmd.PersistentFlags().Bool("debug", false, "debug log")

	cmd.AddCommand(NewWebCommand())
	cmd.AddCommand(NewMigrateCommand())
	cmd.AddCommand(coverage.NewCommand())
	cmd.AddCommand(udm.NewCommand())

	return cmd
}

func Execute() {
	err := New().Execute()
	if err != nil {
		os.Exit(1)
	}
}
