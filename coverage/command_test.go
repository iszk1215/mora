package coverage

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func TestProcessDebugOption_NoFlag(t *testing.T) {
	cmd := &cobra.Command{}
	// No "debug" flag defined - should not panic
	processDebugOption(cmd)
}

func TestUploadCommand_Flags_AreDefined(t *testing.T) {
	cmd := newUploadCommand()

	_, err := cmd.Flags().GetString("server")
	require.NoError(t, err)

	_, err = cmd.Flags().GetString("repo")
	require.NoError(t, err)

	_, err = cmd.Flags().GetString("repo-path")
	require.NoError(t, err)

	_, err = cmd.Flags().GetString("entry")
	require.NoError(t, err)

	_, err = cmd.Flags().GetBool("force")
	require.NoError(t, err)

	_, err = cmd.Flags().GetBool("dry-run")
	require.NoError(t, err)

	_, err = cmd.Flags().GetBool("yes")
	require.NoError(t, err)
}
