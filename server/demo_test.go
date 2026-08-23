package server

import (
	"math/rand"
	"sort"
	"testing"
	"time"

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

func TestDemoValueTimes(t *testing.T) {
	now := time.Date(2026, 8, 24, 12, 0, 0, 0, time.UTC)
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	lower := dayStart.AddDate(0, 0, -89) // oldest possible: today midnight minus 89 days
	upper := dayStart.AddDate(0, 0, 1)   // exclusive upper bound

	t.Run("returns requested count of unique times", func(t *testing.T) {
		for _, xAxisType := range []string{"date", "datetime"} {
			for iter := 0; iter < 500; iter++ {
				rng := rand.New(rand.NewSource(int64(iter)))
				times, err := demoValueTimes(rng, xAxisType, 20, now)
				require.NoError(t, err)
				require.Len(t, times, 20)

				seen := make(map[int64]struct{}, len(times))
				for _, ts := range times {
					_, dup := seen[ts.Unix()]
					require.False(t, dup, "duplicate timestamp %v for %s axis", ts, xAxisType)
					seen[ts.Unix()] = struct{}{}

					require.False(t, ts.Before(lower), "timestamp %v is older than 90 days", ts)
					require.True(t, ts.Before(upper), "timestamp %v is not within today", ts)
				}
			}
		}
	})

	t.Run("sorts date axis times newest first at midnight", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))
		times, err := demoValueTimes(rng, "date", 20, now)
		require.NoError(t, err)
		require.True(t, sort.SliceIsSorted(times, func(i, j int) bool { return times[i].After(times[j]) }))
		for _, ts := range times {
			require.Zero(t, ts.Hour())
			require.Zero(t, ts.Minute())
		}
	})

	t.Run("handles count larger than day range via datetime resampling", func(t *testing.T) {
		rng := rand.New(rand.NewSource(1))
		times, err := demoValueTimes(rng, "datetime", 200, now)
		require.NoError(t, err)
		require.Len(t, times, 200)
		seen := make(map[int64]struct{}, len(times))
		for _, ts := range times {
			seen[ts.Unix()] = struct{}{}
		}
		require.Len(t, seen, 200)
	})
}
