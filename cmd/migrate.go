package cmd

import (
	"fmt"
	"time"

	"github.com/iszk1215/mora/config"
	"github.com/iszk1215/mora/server"
	"github.com/iszk1215/mora/tracker"
	"github.com/iszk1215/mora/udm"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewMigrateCommand() *cobra.Command {
	var migrateCmd = &cobra.Command{
		Use:   "migrate",
		Short: "Run data migrations",
		Long: `Run data migrations.

Currently migrates repository-scoped UDM data (metric -> item -> value) into
repository-independent trackers (tracker -> series -> values). The migration is
non-destructive and runs only once; it is recorded in the schema_migrations
table. Requires the database path from the server config.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if debug, err := cmd.Flags().GetBool("debug"); err == nil && debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}

			configFilename, err := cmd.Flags().GetString("config")
			if err != nil {
				return fmt.Errorf("failed to get config flag: %w", err)
			}

			cfg, err := config.ReadMoraConfig(configFilename)
			if err != nil {
				return err
			}

			cmd.SilenceUsage = true

			db, err := server.OpenDB(cfg)
			if err != nil {
				return fmt.Errorf("open database: %w", err)
			}
			defer func() { _ = db.Close() }()

			if _, err := tracker.NewService(db); err != nil {
				return fmt.Errorf("initialize tracker schema: %w", err)
			}

			start := time.Now()
			if err := udm.MigrateUDMToTracker(db); err != nil {
				return fmt.Errorf("migrate UDM to tracker: %w", err)
			}
			log.Info().Dur("elapsed", time.Since(start)).Msg("migration finished")
			return nil
		},
	}

	migrateCmd.Flags().StringP("config", "c", "mora.conf", "Config filename")

	return migrateCmd
}
