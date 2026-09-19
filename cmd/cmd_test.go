package cmd

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/require"
)

func TestWebCommand_Flags_AreDefined(t *testing.T) {
	cmd := NewWebCommand()

	_, err := cmd.Flags().GetString("config")
	require.NoError(t, err)

	_, err = cmd.Flags().GetBool("debug")
	require.NoError(t, err)

	_, err = cmd.Flags().GetInt("port")
	require.NoError(t, err)
}

func TestMigrateCommand_Flags_AreDefined(t *testing.T) {
	cmd := NewMigrateCommand()

	_, err := cmd.Flags().GetString("config")
	require.NoError(t, err)
}

func TestMigrateCommand_RegisteredInRoot(t *testing.T) {
	cmd := New()

	found := false
	for _, c := range cmd.Commands() {
		if c.Name() == "migrate" {
			found = true
			break
		}
	}
	require.True(t, found)
}

func TestConfigureAccessLogger_MicrosecondTimestamps(t *testing.T) {
	var buf bytes.Buffer
	configureAccessLogger(&buf, true)

	handler := middleware.DefaultLogger(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	req := httptest.NewRequest("GET", "/probe", nil)
	handler.ServeHTTP(httptest.NewRecorder(), req)

	require.Regexp(t, `\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}\.\d{6} `, buf.String())
	require.NotContains(t, buf.String(), "\x1b[")
}
