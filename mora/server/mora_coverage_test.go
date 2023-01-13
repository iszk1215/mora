package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/iszk1215/mora/mora/profile"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUpload(t *testing.T) {
	profile0 := &profile.Profile{
		FileName: "test.go",
		Hits:     13,
		Lines:    17,
		Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 20, 1}},
	}
	profile1 := &profile.Profile{
		FileName: "test2.go",
		Hits:     0,
		Lines:    3,
		Blocks:   [][]int{{1, 3, 0}},
	}
	profiles := []*profile.Profile{profile0, profile1}

	e := &CoverageEntryUploadRequest{
		EntryName: "go",
		Profiles:  profiles,
		Hits:      13,
		Lines:     20,
	}
	entries := []*CoverageEntryUploadRequest{e}

	req := CoverageUploadRequest{
		RepoURL:  "http://mockscm.com/mockowner/mockrepo",
		Revision: "012345",
		Time:     time.Now(),
		Entries:  entries,
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	p := NewMoraCoverageProvider(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
	p.HandleUpload(w, r)

	res := w.Result()

	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Equal(t, 1, len(p.coverages))

	cov := p.coverages[0]
	assert.Equal(t, cov.Revision(), req.Revision)
	require.Equal(t, 1, len(cov.entries))

	entry := cov.entries[0]
	assert.Equal(t, 13, entry.Hits)
	assert.Equal(t, 20, entry.Lines)
	assert.Equal(t, 2, len(entry.profiles))

	require.Equal(t, http.StatusOK, res.StatusCode)
}
