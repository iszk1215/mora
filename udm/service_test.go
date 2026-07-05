package udm

import (
	"testing"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
)

func initTestUDMService(t *testing.T) *Service {
	db, err := sqlx.Connect("sqlite3", ":memory:?_loc=auto")
	require.NoError(t, err)

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
