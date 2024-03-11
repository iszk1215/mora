/*
Copyright © 2022 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"os"

	"github.com/iszk1215/mora/mora/udm"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "mora",
		Short: "Mora is a coverage tracker",
		// Long
	}

	rootCmd.AddCommand(NewWebCommand())
	rootCmd.AddCommand(NewCoverageCommand())
	rootCmd.AddCommand(udm.NewCommand())

	return rootCmd
}

func Execute() {
	err := New().Execute()
	if err != nil {
		os.Exit(1)
	}
}
