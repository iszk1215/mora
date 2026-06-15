package udm

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUDMCommand_Flags_AreDefined(t *testing.T) {
	cmd := NewCommand()

	t.Run("persistent flags", func(t *testing.T) {
		_, err := cmd.PersistentFlags().GetString("config")
		require.NoError(t, err)

		_, err = cmd.PersistentFlags().GetString("server")
		require.NoError(t, err)

		_, err = cmd.PersistentFlags().GetString("repo")
		require.NoError(t, err)

		_, err = cmd.PersistentFlags().GetString("token")
		require.NoError(t, err)

		_, err = cmd.PersistentFlags().GetBool("debug")
		require.NoError(t, err)
	})

	t.Run("metric subcommand flags", func(t *testing.T) {
		metricCmd, _, err := cmd.Find([]string{"metric"})
		require.NoError(t, err)
		require.NotNil(t, metricCmd)

		_, err = metricCmd.Flags().GetBool("create")
		require.NoError(t, err)

		_, err = metricCmd.Flags().GetBool("delete")
		require.NoError(t, err)

		_, err = metricCmd.Flags().GetBool("list")
		require.NoError(t, err)

		_, err = metricCmd.Flags().GetString("type")
		require.NoError(t, err)
	})

	t.Run("value subcommand flags", func(t *testing.T) {
		valueCmd, _, err := cmd.Find([]string{"value"})
		require.NoError(t, err)
		require.NotNil(t, valueCmd)

		_, err = valueCmd.Flags().GetBool("add")
		require.NoError(t, err)

		_, err = valueCmd.Flags().GetBool("list")
		require.NoError(t, err)

		_, err = valueCmd.Flags().GetBool("clear")
		require.NoError(t, err)

		_, err = valueCmd.Flags().GetString("time")
		require.NoError(t, err)
	})
}
