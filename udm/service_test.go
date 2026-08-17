package udm

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/tursodatabase/go-libsql"
	"github.com/stretchr/testify/require"
)

func initTestUDMService(t *testing.T) *Service {
	db, err := sqlx.Connect("libsql", ":memory:")
	require.NoError(t, err)
	 db.MustExec("PRAGMA foreign_keys = OFF")

	svc, err := NewService(db)
	require.NoError(t, err)
	return svc
}

func TestUDMServiceNew(t *testing.T) {
	svc := initTestUDMService(t)
	require.NotNil(t, svc)
}

func TestUDMServiceHandler(t *testing.T) {
	svc := initTestUDMService(t)
	h := svc.Handler()
	require.NotNil(t, h)
}
