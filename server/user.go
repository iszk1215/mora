package server

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/jmoiron/sqlx"
	"github.com/rs/zerolog/log"
)

type User struct {
	ID             int64  `db:"id" json:"id"`
	Provider       string `db:"provider" json:"provider"`
	ProviderUserID string `db:"provider_user_id" json:"provider_user_id"`
	Username       string `db:"username" json:"username"`
	AvatarURL      string `db:"avatar_url" json:"avatar_url"`
	CreatedAt      string `db:"created_at" json:"-"`
	UpdatedAt      string `db:"updated_at" json:"-"`
}

type UserStore interface {
	Init() error
	FindOrCreate(provider, providerUserID, username, avatarURL string) (*User, error)
	FindByID(id int64) (*User, error)
}

type userStore struct {
	db *sqlx.DB
}

func NewUserStore(db *sqlx.DB) UserStore {
	return &userStore{db: db}
}

func (s *userStore) Init() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			provider         TEXT    NOT NULL,
			provider_user_id TEXT    NOT NULL,
			username         TEXT    NOT NULL,
			avatar_url       TEXT    NOT NULL DEFAULT '',
			created_at       TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at       TEXT    NOT NULL DEFAULT (datetime('now')),
			UNIQUE(provider, provider_user_id)
		)
	`)
	if err != nil {
		return err
	}

	// seed superuser (id=1) for API key auth
	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO user (id, provider, provider_user_id, username, avatar_url)
		 VALUES (1, 'system', 'superuser', 'admin', '')`,
	)
	return err
}

func (s *userStore) FindOrCreate(provider, providerUserID, username, avatarURL string) (*User, error) {
	var user User
	err := s.db.Get(&user,
		"SELECT * FROM user WHERE provider = ? AND provider_user_id = ?",
		provider, providerUserID)
	if err == nil {
		_, err = s.db.Exec(
			"UPDATE user SET username = ?, avatar_url = ?, updated_at = datetime('now') WHERE id = ?",
			username, avatarURL, user.ID)
		if err != nil {
			return nil, fmt.Errorf("update user: %w", err)
		}
		user.Username = username
		user.AvatarURL = avatarURL
		return &user, nil
	}

	if err != sql.ErrNoRows {
		return nil, fmt.Errorf("find user: %w", err)
	}

	result, err := s.db.Exec(
		"INSERT INTO user (provider, provider_user_id, username, avatar_url) VALUES (?, ?, ?, ?)",
		provider, providerUserID, username, avatarURL)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	user = User{
		ID:             id,
		Provider:       provider,
		ProviderUserID: providerUserID,
		Username:       username,
		AvatarURL:      avatarURL,
	}
	return &user, nil
}

func (s *userStore) FindByID(id int64) (*User, error) {
	var user User
	err := s.db.Get(&user, "SELECT * FROM user WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type githubUser struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	AvatarURL string `json:"avatar_url"`
}

func fetchGitHubUser(accessToken string) (*githubUser, error) {
	req, err := http.NewRequest("GET", "https://api.github.com/user", nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("github API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var gu githubUser
	if err := json.Unmarshal(body, &gu); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &gu, nil
}

func createUserForSession(sess *MoraSession, rm RepositoryManager, token *scm.Token, userStore UserStore) {
	rmURL := rm.URL().String()

	var provider string
	switch {
	case strings.Contains(rmURL, "github.com"):
		provider = "github"
	default:
		return
	}

	gu, err := fetchGitHubUser(token.Token)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch GitHub user")
		return
	}

	user, err := userStore.FindOrCreate(provider, fmt.Sprintf("%d", gu.ID), gu.Login, gu.AvatarURL)
	if err != nil {
		log.Error().Err(err).Msg("failed to find or create user")
		return
	}

	sess.SetUserID(user.ID)
}
