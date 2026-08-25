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

	"github.com/iszk1215/mora/config"
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

			config_file, err := cmd.Flags().GetString("config")
			if err != nil {
				return fmt.Errorf("failed to get config flag: %w", err)
			}
			debug, err := cmd.Flags().GetBool("debug")
			if err != nil {
				return fmt.Errorf("failed to get debug flag: %w", err)
			}
			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return fmt.Errorf("failed to get port flag: %w", err)
			}
			demo, err := cmd.Flags().GetBool("demo")
			if err != nil {
				return fmt.Errorf("failed to get demo flag: %w", err)
			}
			insecureCookie, err := cmd.Flags().GetBool("insecure-cookie")
			if err != nil {
				return fmt.Errorf("failed to get insecure-cookie flag: %w", err)
			}

			zerolog.SetGlobalLevel(zerolog.InfoLevel)
			if debug {
				zerolog.SetGlobalLevel(zerolog.DebugLevel)
			}

			config, err := config.ReadMoraConfig(config_file)
			if err != nil {
				return err
			}
			config.Debug = debug
			config.Server.Port = port
			config.Demo = demo
			// Only override the config file value when the flag is given
			// explicitly; otherwise insecure_cookie from mora.conf stays intact.
			if cmd.Flags().Changed("insecure-cookie") {
				config.Server.InsecureCookie = insecureCookie
			}
			if demo {
				config.DatabaseFilename = ":memory:"
			}

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

		shutdownDone := make(chan struct{})

		go func() {
			defer close(shutdownDone)
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
			log.Err(err).Msg("server listen failed")
			return fmt.Errorf("ListenAndServe: %w", err)
		}

		<-shutdownDone
		return nil
		},
	}

	webCmd.Flags().BoolP("debug", "d", false, "Enable debug")
	webCmd.Flags().IntP("port", "p", 4000, "port")
	webCmd.Flags().StringP("config", "c", "mora.conf", "Config filename")
	webCmd.Flags().Bool("demo", false, "Start in demo mode with seed data")
	webCmd.Flags().Bool("insecure-cookie", false, "Disable Secure cookie attribute (for development over HTTP)")

	return webCmd
}
