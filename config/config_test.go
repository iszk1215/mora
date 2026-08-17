package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReadMoraConfig_FileNotFound(t *testing.T) {
	_, err := ReadMoraConfig("/nonexistent/path/for/sure/mora.conf")
	require.Error(t, err)
}

func TestReadMoraConfig_InvalidToml(t *testing.T) {
	f, err := os.CreateTemp("", "mora-*.conf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()

	_, err = f.Write([]byte("invalid toml {{{{{"))
	require.NoError(t, err)
	_ = f.Close()

	_, err = ReadMoraConfig(f.Name())
	require.Error(t, err)
}

func TestReadMoraConfig_DefaultDatabaseFilename(t *testing.T) {
	f, err := os.CreateTemp("", "mora-*.conf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()

	_, err = f.Write([]byte(`
[server]
url = "http://localhost:4000"
`))
	require.NoError(t, err)
	_ = f.Close()

	cfg, err := ReadMoraConfig(f.Name())
	require.NoError(t, err)
	require.Equal(t, "mora.db", cfg.DatabaseFilename)
}

func TestReadMoraConfig_FullConfig(t *testing.T) {
	f, err := os.CreateTemp("", "mora-*.conf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()

	_, err = f.Write([]byte(`
database_filename = "test.db"
debug = true

[server]
url = "http://localhost:9090"
port = 9090
sitename = "My Mora"

[client]
server = "http://localhost:9090"
repo = "https://example.com/owner/repo"
token = "sekret"

[[scm]]
scm = "gitea"
url = "https://gitea.example.com"
secret_file = "/tmp/gitea.conf"
insecure_skip_verify = true

[[scm]]
scm = "github"
url = "https://github.com"
secret_file = "/tmp/github.conf"
`))
	require.NoError(t, err)
	_ = f.Close()

	cfg, err := ReadMoraConfig(f.Name())
	require.NoError(t, err)

	require.Equal(t, "test.db", cfg.DatabaseFilename)
	require.True(t, cfg.Debug)

	require.Equal(t, "http://localhost:9090", cfg.Server.URL)
	require.Equal(t, 9090, cfg.Server.Port)
	require.Equal(t, "My Mora", cfg.Server.SiteName)

	require.Len(t, cfg.RepositoryManagers, 2)
	require.Equal(t, "gitea", cfg.RepositoryManagers[0].Driver)
	require.Equal(t, "https://gitea.example.com", cfg.RepositoryManagers[0].URL)
	require.Equal(t, "/tmp/gitea.conf", cfg.RepositoryManagers[0].SecretFilename)
	require.True(t, cfg.RepositoryManagers[0].InsecureSkipVerify)
	require.Equal(t, "github", cfg.RepositoryManagers[1].Driver)
	require.Equal(t, "https://github.com", cfg.RepositoryManagers[1].URL)
	require.Equal(t, "/tmp/github.conf", cfg.RepositoryManagers[1].SecretFilename)
	require.False(t, cfg.RepositoryManagers[1].InsecureSkipVerify)

	require.Equal(t, "http://localhost:9090", cfg.Client.ServerURL)
	require.Equal(t, "https://example.com/owner/repo", cfg.Client.RepositoryURL)
	require.Equal(t, "sekret", cfg.Client.Token)
}

func TestReadClientConfig_Success(t *testing.T) {
	f, err := os.CreateTemp("", "mora-*.conf")
	require.NoError(t, err)
	defer func() { _ = os.Remove(f.Name()) }()

	_, err = f.Write([]byte(`
[client]
server = "http://localhost:9090"
repo = "https://example.com/owner/repo"
token = "sekret"
`))
	require.NoError(t, err)
	_ = f.Close()

	cc, err := ReadClientConfig(f.Name())
	require.NoError(t, err)
	require.Equal(t, "http://localhost:9090", cc.ServerURL)
	require.Equal(t, "https://example.com/owner/repo", cc.RepositoryURL)
	require.Equal(t, "sekret", cc.Token)
}

func TestReadClientConfig_Error(t *testing.T) {
	_, err := ReadClientConfig("/nonexistent/path/for/sure/mora.conf")
	require.Error(t, err)
	require.Contains(t, err.Error(), "ReadClientConfig")
}
