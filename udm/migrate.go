package udm

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

// UdmTrackerOwnerID is the user that owns trackers created by the UDM
// migration. The admin user (id=1) is the superuser.
const UdmTrackerOwnerID int64 = 1

const migrationNameUDMToTracker = "udm_to_tracker"

var schemaMigrations = `
CREATE TABLE IF NOT EXISTS schema_migrations (
    name TEXT PRIMARY KEY,
    applied_at DATETIME NOT NULL
)`

func tableExists(db *sqlx.DB, name string) (bool, error) {
	var count int
	err := db.Get(&count,
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", name)
	if err != nil {
		return false, fmt.Errorf("tableExists(%s): %w", name, err)
	}
	return count > 0, nil
}

func migrationApplied(db *sqlx.DB, name string) (bool, error) {
	var count int
	err := db.Get(&count,
		"SELECT COUNT(*) FROM schema_migrations WHERE name = ?", name)
	if err != nil {
		return false, fmt.Errorf("migrationApplied(%s): %w", name, err)
	}
	return count > 0, nil
}

// MigrateUDMToTracker migrates repository-scoped UDM data
// (udm_metric -> udm_item -> udm_value) into repository-independent trackers
// (tracker -> tracker_series -> tracker_value).
//
// Mapping:
//   - each udm_metric row becomes a tracker: name = metric.name,
//     visibility = "public", type = "tracker", owner_id = UdmTrackerOwnerID
//     (admin). created_at/last_updated_at are taken from the min/max value
//     timestamps (or now when the metric has no values).
//   - each udm_item row becomes a tracker_series: name = item.name,
//     data_type = "int" (the only UDM value type), config = "{}".
//   - each udm_value row becomes a tracker_value: value TEXT is parsed to
//     float64; unparsable values are skipped with a warning.
//
// The migration is non-destructive (UDM tables are left intact) and
// idempotent: the applied state is recorded in the schema_migrations table, so
// running it again is a no-op. The tracker tables must already exist; the
// `mora migrate` command creates them via tracker.NewService.
func MigrateUDMToTracker(db *sqlx.DB) error {
	if _, err := db.Exec(schemaMigrations); err != nil {
		return fmt.Errorf("MigrateUDMToTracker schema_migrations: %w", err)
	}

	applied, err := migrationApplied(db, migrationNameUDMToTracker)
	if err != nil {
		return err
	}
	if applied {
		log.Print("udm.MigrateUDMToTracker: already applied, skipping")
		return nil
	}

	hasTracker, err := tableExists(db, "tracker")
	if err != nil {
		return err
	}
	if !hasTracker {
		return fmt.Errorf("MigrateUDMToTracker: tracker table does not exist, initialize the tracker schema first")
	}

	// UDM tables missing means there is nothing to migrate.
	hasMetric, err := tableExists(db, "udm_metric")
	if err != nil {
		return err
	}
	hasItem, err := tableExists(db, "udm_item")
	if err != nil {
		return err
	}
	hasValue, err := tableExists(db, "udm_value")
	if err != nil {
		return err
	}
	if !hasMetric || !hasItem || !hasValue {
		log.Print("udm.MigrateUDMToTracker: UDM tables not found, nothing to migrate")
		return recordMigration(db, migrationNameUDMToTracker)
	}

	tx, err := db.Beginx()
	if err != nil {
		return fmt.Errorf("MigrateUDMToTracker begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var metrics []struct {
		Id   int64  `db:"id"`
		Name string `db:"name"`
	}
	if err := tx.Select(&metrics, "SELECT id, name FROM udm_metric ORDER BY id"); err != nil {
		return fmt.Errorf("MigrateUDMToTracker select metrics: %w", err)
	}

	var trackerCount, seriesCount, valueCount int

	for _, metric := range metrics {
		trackerID, createdAt, lastUpdatedAt, err := insertMigratedTracker(tx, metric.Id, metric.Name)
		if err != nil {
			return err
		}
		trackerCount++

		var items []struct {
			Id   int64  `db:"id"`
			Name string `db:"name"`
		}
		if err := tx.Select(&items, "SELECT id, name FROM udm_item WHERE metric_id = ? ORDER BY id", metric.Id); err != nil {
			return fmt.Errorf("MigrateUDMToTracker select items: %w", err)
		}

		for _, item := range items {
			seriesID, err := insertMigratedSeries(tx, trackerID, item.Name)
			if err != nil {
				return err
			}
			seriesCount++

			var values []struct {
				Time  time.Time `db:"time"`
				Value string    `db:"value"`
			}
			if err := tx.Select(&values, "SELECT time, value FROM udm_value WHERE item_id = ? ORDER BY time", item.Id); err != nil {
				return fmt.Errorf("MigrateUDMToTracker select values: %w", err)
			}

			for _, value := range values {
				f, err := strconv.ParseFloat(value.Value, 64)
				if err != nil {
					log.Warn().Int64("item_id", item.Id).Str("value", value.Value).
						Err(err).Msg("MigrateUDMToTracker: skipping unparsable value")
					continue
				}
				if _, err := tx.Exec(
					"INSERT INTO tracker_value (series_id, time, value) VALUES (?, ?, ?)",
					seriesID, value.Time, f); err != nil {
					return fmt.Errorf("MigrateUDMToTracker insert tracker_value: %w", err)
				}
				valueCount++
			}
		}

		log.Debug().Int64("tracker_id", trackerID).Str("name", metric.Name).
			Time("created_at", createdAt).Time("last_updated_at", lastUpdatedAt).
			Msg("MigrateUDMToTracker: tracker created")
	}

	if err := recordMigrationTx(tx, migrationNameUDMToTracker); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("MigrateUDMToTracker commit: %w", err)
	}

	log.Info().
		Int("trackers", trackerCount).
		Int("series", seriesCount).
		Int("values", valueCount).
		Msg("MigrateUDMToTracker: UDM data migrated to trackers")

	return nil
}

// insertMigratedTracker creates a tracker for a udm_metric row and returns its
// id plus the created_at/last_updated_at timestamps derived from the metric's
// value times.
func insertMigratedTracker(tx *sqlx.Tx, metricID int64, name string) (int64, time.Time, time.Time, error) {
	createdAt, lastUpdatedAt, err := metricTimeRange(tx, metricID)
	if err != nil {
		return 0, time.Time{}, time.Time{}, err
	}

	res, err := tx.Exec(`
		INSERT INTO tracker (name, description, body, visibility, type, chart_config, owner_id, created_at, last_updated_at)
		VALUES (?, '', '', 'public', 'tracker', '{}', ?, ?, ?)`,
		name, UdmTrackerOwnerID, createdAt, lastUpdatedAt)
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("insertMigratedTracker insert: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, time.Time{}, time.Time{}, fmt.Errorf("insertMigratedTracker LastInsertId: %w", err)
	}

	return id, createdAt, lastUpdatedAt, nil
}

// insertMigratedSeries creates a tracker_series for a udm_item row and returns
// the series id.
func insertMigratedSeries(tx *sqlx.Tx, trackerID int64, name string) (int64, error) {
	res, err := tx.Exec(`
		INSERT INTO tracker_series (tracker_id, name, data_type, config)
		VALUES (?, ?, 'int', '{}')`,
		trackerID, name)
	if err != nil {
		return 0, fmt.Errorf("insertMigratedSeries insert: %w", err)
	}

	id, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("insertMigratedSeries LastInsertId: %w", err)
	}

	return id, nil
}

// metricTimeRange returns the min/max value timestamps across all items of a
// metric, or now when the metric has no values. The times are selected as
// plain DATETIME columns (an SQL MIN/MAX expression loses the declared column
// type and comes back as a string).
func metricTimeRange(tx *sqlx.Tx, metricID int64) (time.Time, time.Time, error) {
	var times []time.Time
	err := tx.Select(&times, `
		SELECT uv.time
		FROM udm_value uv
		JOIN udm_item ui ON ui.id = uv.item_id
		WHERE ui.metric_id = ?
		ORDER BY uv.time`, metricID)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("metricTimeRange select: %w", err)
	}

	now := time.Now()
	if len(times) == 0 {
		return now, now, nil
	}
	return times[0], times[len(times)-1], nil
}

func recordMigration(db *sqlx.DB, name string) error {
	if _, err := db.Exec(
		"INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)",
		name, time.Now()); err != nil {
		return fmt.Errorf("recordMigration(%s): %w", name, err)
	}
	return nil
}

func recordMigrationTx(tx *sqlx.Tx, name string) error {
	if _, err := tx.Exec(
		"INSERT INTO schema_migrations (name, applied_at) VALUES (?, ?)",
		name, time.Now()); err != nil {
		return fmt.Errorf("recordMigrationTx(%s): %w", name, err)
	}
	return nil
}
