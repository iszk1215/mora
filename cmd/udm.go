/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>

*/
package cmd

import (
	"github.com/iszk1215/mora/mora/udm"
)

func init() {
	rootCmd.AddCommand(udm.NewCommand())
}
