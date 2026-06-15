package cmd

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWebCommand_Flags_AreDefined(t *testing.T) {
	cmd := NewWebCommand()

	_, err := cmd.Flags().GetString("config")
	require.NoError(t, err)

	_, err = cmd.Flags().GetBool("debug")
	require.NoError(t, err)

	_, err = cmd.Flags().GetInt("port")
	require.NoError(t, err)
}
