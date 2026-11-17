package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBaseRepositoryManager_Driver(t *testing.T) {
	var b BaseRepositoryManager
	b.Init(1, "scm_test", nil, nil, nil, "gitea")
	require.Equal(t, "gitea", b.Driver())
}
