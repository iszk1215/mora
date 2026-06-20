package cmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
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

		go func() {
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			<-sigCh

			log.Info().Msg("Shutting down...")
			if err := server.Close(); err != nil {
				log.Error().Err(err).Msg("server.Close")
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := srv.Shutdown(ctx); err != nil {
				log.Error().Err(err).Msg("srv.Shutdown")
			}
		}()

		err = srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Err(err).Msg("")
			return fmt.Errorf("ListenAndServe: %w", err)
		}

		return nil
		},
	}

	webCmd.Flags().BoolP("debug", "d", false, "Enable debug")
	webCmd.Flags().IntP("port", "p", 4000, "port")
	webCmd.Flags().StringP("config", "c", "mora.conf", "Config filename")

	return webCmd
}
