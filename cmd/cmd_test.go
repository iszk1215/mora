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

func TestMigrateCommand_Flags_AreDefined(t *testing.T) {
	cmd := NewMigrateCommand()

	_, err := cmd.Flags().GetString("config")
	require.NoError(t, err)
}

func TestMigrateCommand_RegisteredInRoot(t *testing.T) {
	cmd := New()

	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "migrate" {
			found = true
			break
		}
	}
	require.True(t, found)
}
