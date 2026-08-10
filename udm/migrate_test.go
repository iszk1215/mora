package udm

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/tracker"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

type migratedTrackerRow struct {
	Id            int64     `db:"id"`
	Name          string    `db:"name"`
	Description   string    `db:"description"`
	Body          string    `db:"body"`
	Visibility    string    `db:"visibility"`
	Type          string    `db:"type"`
	ChartConfig   string    `db:"chart_config"`
	OwnerId       int64     `db:"owner_id"`
	CreatedAt     time.Time `db:"created_at"`
	LastUpdatedAt time.Time `db:"last_updated_at"`
}

type migratedSeriesRow struct {
	Id        int64  `db:"id"`
	TrackerId int64  `db:"tracker_id"`
	Name      string `db:"name"`
	DataType  string `db:"data_type"`
	Config    string `db:"config"`
}

type migratedValueRow struct {
	Id       int64     `db:"id"`
	SeriesId int64     `db:"series_id"`
	Time     time.Time `db:"time"`
	Value    float64   `db:"value"`
}

func initMigrateTestStore(t *testing.T) *sqlx.DB {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	db.MustExec("PRAGMA foreign_keys = ON")

	// Create user table and seed admin (superuser) for FK on tracker.owner_id.
	db.MustExec(`
		CREATE TABLE user (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			username TEXT NOT NULL,
			avatar_url TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(provider, provider_user_id)
		)
	`)
	db.MustExec(`INSERT INTO user (id, provider, provider_user_id, username, avatar_url)
		VALUES (1, 'system', 'superuser', 'admin', '')`)

	udmStore := newUdmStore(db)
	require.NoError(t, udmStore.initialize())

	_, err = tracker.NewService(db)
	require.NoError(t, err)

	return db
}

func TestMigrateUDMToTracker(t *testing.T) {
	db := initMigrateTestStore(t)
	us := newUdmStore(db)

	t1 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t3 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)

	initMockStore(t, us, &storeInitializer{
		repoId: 1,
		metrics: []*metricInitializer{
			{
				metric: &MetricModel{Name: "cpu"},
				items: []*itemInitializer{
					{
						item: &ItemModel{Name: "user", ValueType: ValueTypeInt},
						values: []*ValueModel{
							{Timestamp: t1, Value: "10"},
							{Timestamp: t3, Value: "30"},
						},
					},
					{
						item: &ItemModel{Name: "system", ValueType: ValueTypeInt},
						values: []*ValueModel{
							{Timestamp: t2, Value: "20"},
						},
					},
				},
			},
		},
	})

	require.NoError(t, MigrateUDMToTracker(db))

	var trk migratedTrackerRow
	err := db.Get(&trk, "SELECT id, name, description, body, visibility, type, chart_config, owner_id, created_at, last_updated_at FROM tracker")
	require.NoError(t, err)
	require.Equal(t, "cpu", trk.Name)
	require.Empty(t, trk.Description)
	require.Empty(t, trk.Body)
	require.Equal(t, "public", trk.Visibility)
	require.Equal(t, "tracker", trk.Type)
	require.Equal(t, "{}", trk.ChartConfig)
	require.Equal(t, UdmTrackerOwnerID, trk.OwnerId)
	require.Equal(t, UdmTrackerOwnerID, int64(1))
	require.True(t, trk.CreatedAt.Equal(t1), "created_at should be min value time")
	require.True(t, trk.LastUpdatedAt.Equal(t3), "last_updated_at should be max value time")

	var series []migratedSeriesRow
	err = db.Select(&series, "SELECT id, tracker_id, name, data_type, config FROM tracker_series ORDER BY id")
	require.NoError(t, err)
	require.Len(t, series, 2)
	require.Equal(t, "user", series[0].Name)
	require.Equal(t, "int", series[0].DataType)
	require.Equal(t, "{}", series[0].Config)
	require.Equal(t, "system", series[1].Name)
	require.Equal(t, "int", series[1].DataType)

	var values []migratedValueRow
	err = db.Select(&values, "SELECT id, series_id, time, value FROM tracker_value ORDER BY time")
	require.NoError(t, err)
	require.Len(t, values, 3)
	require.Equal(t, 10.0, values[0].Value)
	require.Equal(t, 20.0, values[1].Value)
	require.Equal(t, 30.0, values[2].Value)
	require.True(t, values[0].Time.Equal(t1))
	require.True(t, values[2].Time.Equal(t3))

	// UDM tables must be left intact.
	var metricCount, itemCount, valueCount int
	require.NoError(t, db.Get(&metricCount, "SELECT COUNT(*) FROM udm_metric"))
	require.NoError(t, db.Get(&itemCount, "SELECT COUNT(*) FROM udm_item"))
	require.NoError(t, db.Get(&valueCount, "SELECT COUNT(*) FROM udm_value"))
	require.Equal(t, 1, metricCount)
	require.Equal(t, 2, itemCount)
	require.Equal(t, 3, valueCount)
}

func TestMigrateUDMToTrackerIdempotent(t *testing.T) {
	db := initMigrateTestStore(t)
	us := newUdmStore(db)

	initMockStore(t, us, &storeInitializer{
		repoId: 1,
		metrics: []*metricInitializer{
			{
				metric: &MetricModel{Name: "cpu"},
				items: []*itemInitializer{
					{
						item:   &ItemModel{Name: "user", ValueType: ValueTypeInt},
						values: []*ValueModel{{Timestamp: time.Now(), Value: "10"}},
					},
				},
			},
		},
	})

	require.NoError(t, MigrateUDMToTracker(db))
	require.NoError(t, MigrateUDMToTracker(db))

	var trackerCount int
	require.NoError(t, db.Get(&trackerCount, "SELECT COUNT(*) FROM tracker"))
	require.Equal(t, 1, trackerCount)

	var seriesCount int
	require.NoError(t, db.Get(&seriesCount, "SELECT COUNT(*) FROM tracker_series"))
	require.Equal(t, 1, seriesCount)

	var valueCount int
	require.NoError(t, db.Get(&valueCount, "SELECT COUNT(*) FROM tracker_value"))
	require.Equal(t, 1, valueCount)
}

func TestMigrateUDMToTrackerSameNameMetricsBecomeSeparateTrackers(t *testing.T) {
	db := initMigrateTestStore(t)
	us := newUdmStore(db)

	now := time.Now()

	initMockStore(t, us, &storeInitializer{
		repoId: 1,
		metrics: []*metricInitializer{
			{
				metric: &MetricModel{Name: "cpu"},
				items: []*itemInitializer{
					{
						item:   &ItemModel{Name: "user", ValueType: ValueTypeInt},
						values: []*ValueModel{{Timestamp: now, Value: "10"}},
					},
				},
			},
		},
	})
	initMockStore(t, us, &storeInitializer{
		repoId: 2,
		metrics: []*metricInitializer{
			{
				metric: &MetricModel{Name: "cpu"},
				items: []*itemInitializer{
					{
						item:   &ItemModel{Name: "user", ValueType: ValueTypeInt},
						values: []*ValueModel{{Timestamp: now, Value: "20"}},
					},
				},
			},
		},
	})

	require.NoError(t, MigrateUDMToTracker(db))

	var names []string
	require.NoError(t, db.Select(&names, "SELECT name FROM tracker ORDER BY id"))
	require.Equal(t, []string{"cpu", "cpu"}, names)

	var trackerCount int
	require.NoError(t, db.Get(&trackerCount, "SELECT COUNT(*) FROM tracker"))
	require.Equal(t, 2, trackerCount)
}

func TestMigrateUDMToTrackerSkipsUnparsableValue(t *testing.T) {
	db := initMigrateTestStore(t)
	us := newUdmStore(db)

	initMockStore(t, us, &storeInitializer{
		repoId: 1,
		metrics: []*metricInitializer{
			{
				metric: &MetricModel{Name: "cpu"},
				items: []*itemInitializer{
					{
						item: &ItemModel{Name: "user", ValueType: ValueTypeInt},
						values: []*ValueModel{
							{Timestamp: time.Now(), Value: "10"},
							{Timestamp: time.Now(), Value: "not-a-number"},
						},
					},
				},
			},
		},
	})

	require.NoError(t, MigrateUDMToTracker(db))

	var valueCount int
	require.NoError(t, db.Get(&valueCount, "SELECT COUNT(*) FROM tracker_value"))
	require.Equal(t, 1, valueCount)
}

func TestMigrateUDMToTrackerNoUdmTables(t *testing.T) {
	db := initMigrateTestStore(t)

	require.NoError(t, MigrateUDMToTracker(db))

	var markerCount int
	require.NoError(t, db.Get(&markerCount, "SELECT COUNT(*) FROM schema_migrations WHERE name = ?", migrationNameUDMToTracker))
	require.Equal(t, 1, markerCount)

	// Running again is still a no-op.
	require.NoError(t, MigrateUDMToTracker(db))
}

func TestMigrateUDMToTrackerNoTrackerTables(t *testing.T) {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

	udmStore := newUdmStore(db)
	require.NoError(t, udmStore.initialize())

	err = MigrateUDMToTracker(db)
	require.Error(t, err)
	require.ErrorContains(t, err, "tracker table does not exist")
}
