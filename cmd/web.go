package cmd

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/iszk1215/mora/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

func NewWebCommand() *cobra.Command {

	var webCmd = &cobra.Command{
		Use:   "web",
		Short: "Start mora web server",

		RunE: func(cmd *cobra.Command, args []string) error {
			log.Logger = log.Output(
				zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339}).With().Caller().Logger()

			config_file, _ := cmd.Flags().GetString("config")
			debug, _ := cmd.Flags().GetBool("debug")
			port, _ := cmd.Flags().GetInt("port")

			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			if debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}

			config, err := server.ReadMoraConfig(config_file)
			if err != nil {
				return err
			}
			config.Debug = debug
			config.Server.Port = port

			server, err := server.NewMoraServerFromConfig(config)
			if err != nil {
				return err
			}

			handler := server.Handler()

			srv := &http.Server{
				Addr:         ":" + strconv.Itoa(config.Server.Port),
				Handler:      handler,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  120 * time.Second,
			}

			log.Info().Msg("Started")
			err = srv.ListenAndServe()
			if err != nil {
				log.Err(err).Msg("")
				return err
			}

			return nil
		},
	}

	webCmd.Flags().BoolP("debug", "d", false, "Enable debug")
	webCmd.Flags().IntP("port", "p", 4000, "port")
	webCmd.Flags().StringP("config", "c", "mora.conf", "Config filename")

	return webCmd
}
