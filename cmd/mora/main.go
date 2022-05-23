package main

import (
	"flag"
	"net/http"
	"os"
	"time"

	"github.com/iszk1215/mora"
	"github.com/rs/zerolog"

	"github.com/rs/zerolog/log"
)

func main() {
	log.Logger = log.Output(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339})

	debug := flag.Bool("debug", false, "sets log level to debug")
	config_file := flag.String("config", "mora.yaml", "sets log level to debug")
	flag.Parse()
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if *debug {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	config, err := mora.ReadMoraConfig(*config_file)
	if err != nil {
		log.Err(err).Msg(*config_file)
		os.Exit(1)
	}
	config.Debug = *debug

	server, err := mora.NewMoraServerFromConfig(config)
	if err != nil {
		log.Err(err).Msg("")
		os.Exit(1)
	}

	// handler, err := mora.ServerHandlerFromConfig(config)
	handler, err := server.Handler()
	if err != nil {
		log.Err(err).Msg("")
		os.Exit(1)
	}

	log.Info().Msg("Started")
	err = http.ListenAndServe(":"+config.Port, handler)
	if err != nil {
		log.Err(err).Msg("")
	}
}
