package server

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/iszk1215/mora/config"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"

	_ "github.com/tursodatabase/go-libsql"
)

// OpenDB opens a database connection according to the configuration.
//
// When TursoURL (or the TURSO_DATABASE_URL environment variable) is set, the
// database is a remote libSQL database (Turso Cloud or a self-hosted
// libsql-server). Otherwise the database is a local libSQL file, which is
// compatible with SQLite files such as an existing mora.db.
func OpenDB(cfg config.MoraConfig) (*sqlx.DB, error) {
	log.Info().Msgf("Initialize store: filename=%s", cfg.DatabaseFilename)

	var dsn string
	var remote bool

	tursoURL := cfg.TursoURL
	if tursoURL == "" {
		tursoURL = os.Getenv("TURSO_DATABASE_URL")
	}
	if tursoURL != "" {
		authToken := cfg.TursoAuthToken
		if authToken == "" {
			authToken = os.Getenv("TURSO_AUTH_TOKEN")
		}
		var err error
		dsn, err = remoteDSN(tursoURL, authToken)
		if err != nil {
			return nil, err
		}
		remote = true
	} else {
		dsn = localDSN(cfg.DatabaseFilename)
	}

	db, err := sqlx.Connect("libsql", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect database: %w", err)
	}

	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		if remote {
			log.Warn().Err(err).Msg("PRAGMA foreign_keys failed on remote database; relying on server-side enforcement")
		} else {
			_ = db.Close()
			return nil, fmt.Errorf("PRAGMA foreign_keys: %w", err)
		}
	}

	// journal_mode and busy_timeout are local-file settings only. Both pragmas
	// return a result row, so they must be executed through Query.
	if !remote && !isMemoryDSN(dsn) {
		if rows, err := db.Query("PRAGMA journal_mode=WAL"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("PRAGMA journal_mode: %w", err)
		} else {
			_ = rows.Close()
		}
		if rows, err := db.Query("PRAGMA busy_timeout=5000"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("PRAGMA busy_timeout: %w", err)
		} else {
			_ = rows.Close()
		}
	}

	db.SetMaxOpenConns(1)

	return db, nil
}

// remoteDSN builds the driver connection string for a remote libSQL database,
// appending the auth token as a query parameter.
func remoteDSN(baseURL, authToken string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse Turso URL %q: %w", baseURL, err)
	}
	switch u.Scheme {
	case "libsql", "https", "http":
	default:
		return "", fmt.Errorf("unsupported Turso URL scheme %q (expected libsql:// or https://)", u.Scheme)
	}
	if authToken != "" {
		q := u.Query()
		q.Set("authToken", authToken)
		u.RawQuery = q.Encode()
	}
	return u.String(), nil
}

// localDSN converts a filename into a driver connection string. libSQL only
// accepts URLs with a file: scheme (or :memory:), so bare paths are prefixed.
func localDSN(filename string) string {
	if filename == "" || filename == ":memory:" {
		return ":memory:"
	}
	if strings.HasPrefix(filename, "file:") {
		return filename
	}
	return "file:" + filename
}

func isMemoryDSN(dsn string) bool {
	return strings.HasPrefix(dsn, ":memory:") || strings.HasPrefix(dsn, "file::memory:")
}
