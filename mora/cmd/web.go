/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/iszk1215/mora/mora/server"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
)

// webCmd represents the web command
var webCmd = &cobra.Command{
	Use:   "web",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
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

		log.Info().Msg("Started")
		err = http.ListenAndServe(":"+strconv.Itoa(config.Server.Port), handler)
		if err != nil {
			log.Err(err).Msg("")
			return err
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(webCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// webCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// webCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	webCmd.Flags().BoolP("debug", "d", false, "Enable debug")
	webCmd.Flags().IntP("port", "p", 4000, "port")
	webCmd.Flags().StringP("config", "c", "mora.conf", "Config filename")
}
