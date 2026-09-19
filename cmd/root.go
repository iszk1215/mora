package cmd

import (
	"os"

	"github.com/iszk1215/mora/coverage"
	"github.com/iszk1215/mora/udm"
	"github.com/iszk1215/mora/version"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// logTimestampFormatUs is the microsecond-precision timestamp format used for
// both the zerolog time field encoding and the console output. It matches the
// standard library access log format used by chi.
const logTimestampFormatUs = "2006/01/02 15:04:05.000000"

func New() *cobra.Command {
	noColor := false
	o, _ := os.Stderr.Stat()
	if (o.Mode() & os.ModeCharDevice) != os.ModeCharDevice {
		noColor = true
	}

	zerolog.TimeFieldFormat = logTimestampFormatUs

	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: logTimestampFormatUs, NoColor: noColor}).With().Caller().Logger()

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
