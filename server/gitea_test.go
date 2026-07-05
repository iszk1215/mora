package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGitea_RevisionURL(t *testing.T) {
	g := &Gitea{}
	got := g.RevisionURL("https://gitea.example.com/owner/repo", "abc123")
	require.Equal(t, "https://gitea.example.com/owner/repo/src/commit/abc123", got)
}
