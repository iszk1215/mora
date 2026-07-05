package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestTrackerNamesForUser(t *testing.T) {
	t.Run("returns first N names for index 0", func(t *testing.T) {
		names := trackerNamesForUser(0, 3)
		require.Len(t, names, 3)
		require.Equal(t, "Project Alpha", names[0])
		require.Equal(t, "Project Beta", names[1])
		require.Equal(t, "Project Gamma", names[2])
	})

	t.Run("wraps around at end of list", func(t *testing.T) {
		names := trackerNamesForUser(4, 3)
		require.Len(t, names, 3)
	})

	t.Run("stops at end when count exceeds remaining", func(t *testing.T) {
		// start = (49 * 10) % 50 = 490 % 50 = 40, so we get 10 names (40-49)
		names := trackerNamesForUser(49, 20)
		require.Len(t, names, 10)
	})

	t.Run("returns all names for full span", func(t *testing.T) {
		names := trackerNamesForUser(0, 100)
		require.Len(t, names, 50)
	})
}
