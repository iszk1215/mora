package mora

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleUpload(t *testing.T) {
	profile0 := &Profile{
		FileName: "test.go",
		Hits:     12,
		Lines:    15,
		Blocks:   [][]int{{1, 5, 1}, {10, 13, 0}, {13, 30, 1}},
	}
	profile1 := &Profile{
		FileName: "test2.go",
		Hits:     0,
		Lines:    3,
		Blocks:   [][]int{{1, 3, 0}},
	}
	profiles := []*Profile{profile0, profile1}

	req := CoverageUploadRequest{
		Format:    "go",
		EntryName: "go",
		RepoURL:   "http://mockscm.com/mockowner/mockrepo",
		Revision:  "012345",
		Prefix:    "mockscm.com/mockowner/mockrepo",
		Time:      time.Now(),
		Profiles:  profiles,
	}

	body, err := json.Marshal(req)
	require.NoError(t, err)

	p := NewToolCoverageProvider(nil)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewBuffer(body))
	p.HandleUpload(w, r)

	res := w.Result()

	covs, ok := p.covmap[req.RepoURL]
	require.True(t, ok)
	require.Equal(t, 1, len(covs))

	cov, ok := covs[0].(*coverageImpl)
	assert.True(t, ok)
	assert.Equal(t, cov.Revision(), req.Revision)
	require.Equal(t, 1, len(cov.entries))

	entry := cov.entries[0]
	assert.Equal(t, 12, entry.hits)
	assert.Equal(t, 18, entry.lines)
	assert.Equal(t, 2, len(entry.profiles))

	require.Equal(t, http.StatusOK, res.StatusCode)
}
