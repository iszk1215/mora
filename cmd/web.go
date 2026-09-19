package cmd

import (
	"context"
	"errors"
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5/middleware"
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
			zerolog.TimeFieldFormat = logTimestampFormatMs
			noColor := false
			if o, err := os.Stderr.Stat(); err == nil && (o.Mode()&os.ModeCharDevice) != os.ModeCharDevice {
				noColor = true
			}
			log.Logger = log.Output(
				zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: logTimestampFormatMs, NoColor: noColor}).With().Caller().Logger()
			// chi's access logger is created in init() with hardcoded flags;
			// rewire the exported DefaultLogger so it matches the ms-precision
			// timestamps of the zerolog output and disables colors off-TTY.
			configureAccessLogger(os.Stderr, noColor)

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

// configureAccessLogger rewires chi's exported DefaultLogger hook so the
// access log uses microsecond timestamps and writes to the given output. chi
// creates its own logger in init() with second-precision flags, so log flag
// configuration on the standard logger has no effect on it.
func configureAccessLogger(out io.Writer, noColor bool) {
	middleware.DefaultLogger = middleware.RequestLogger(&middleware.DefaultLogFormatter{
		Logger:  stdlog.New(out, "", stdlog.LstdFlags|stdlog.Lmicroseconds),
		NoColor: noColor,
	})
}
