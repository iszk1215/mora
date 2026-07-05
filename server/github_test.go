package server

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGithub_RevisionURL(t *testing.T) {
	g := &Github{}
	got := g.RevisionURL("https://github.com/owner/repo", "abc123")
	require.Equal(t, "https://github.com/owner/repo/tree/abc123", got)
}
