package server

import (
	"testing"
	"time"

	"github.com/iszk1215/mora/config"
	"github.com/stretchr/testify/require"
)

// TestOpenDB_MemoryDSNOutlivesConnectionRecycle covers the reason memory DSNs
// opt out of connection recycling: the schema lives inside the connection, so
// recycling it does not cost a round trip, it deletes the database. Demo mode
// runs on ":memory:", and its session GC would otherwise fail with
// "no such table: session" the first time the pool went idle.
func TestOpenDB_MemoryDSNOutlivesConnectionRecycle(t *testing.T) {
	// Keep a stray TURSO_DATABASE_URL in the developer's environment from
	// turning this into a remote-database test.
	t.Setenv("TURSO_DATABASE_URL", "")

	origIdle, origLifetime := connMaxIdleTime, connMaxLifetime
	connMaxIdleTime, connMaxLifetime = 10*time.Millisecond, 0
	t.Cleanup(func() { connMaxIdleTime, connMaxLifetime = origIdle, origLifetime })

	db, err := OpenDB(config.MoraConfig{DatabaseFilename: ":memory:"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	db.MustExec("CREATE TABLE mem (x int)")
	db.MustExec("INSERT INTO mem VALUES (1)")

	// database/sql's cleaner only wakes up every 500ms at the shortest, so
	// wait long enough for it to run several times over the shortened window.
	time.Sleep(1500 * time.Millisecond)

	var n int
	require.NoError(t, db.Get(&n, "SELECT count(*) FROM mem"))
	require.Equal(t, 1, n)
}
