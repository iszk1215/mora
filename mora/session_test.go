package mora

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSessionManager(t *testing.T) {
	m, err := NewMoraSessionManager()
	require.NoError(t, err)
	next := func(w http.ResponseWriter, r *http.Request) {
		_, ok := MoraSessionFrom(r.Context())
		require.True(t, ok)
	}
	handler := m.SessionMiddleware(http.HandlerFunc(next))
	req := httptest.NewRequest(http.MethodGet, "/", strings.NewReader(""))
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
}
