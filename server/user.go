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
	ID        int64  `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	AvatarURL string `db:"avatar_url" json:"avatar_url"`
	CreatedAt string `db:"created_at" json:"-"`
	UpdatedAt string `db:"updated_at" json:"-"`
}

type UserAuth struct {
	ID             int64  `db:"id" json:"-"`
	UserID         int64  `db:"user_id" json:"user_id"`
	Provider       string `db:"provider" json:"provider"`
	ProviderUserID string `db:"provider_user_id" json:"provider_user_id"`
}

type UserStore interface {
	Init() error
	FindByProvider(provider, providerUserID string) (*User, error)
	CreateUser(username, avatarURL string) (*User, error)
	LinkAuth(userID int64, provider, providerUserID string) error
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
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			username   TEXT    NOT NULL,
			avatar_url TEXT    NOT NULL DEFAULT '',
			created_at TEXT    NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT    NOT NULL DEFAULT (datetime('now'))
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_auth (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id          INTEGER NOT NULL REFERENCES user(id),
			provider         TEXT    NOT NULL,
			provider_user_id TEXT    NOT NULL,
			UNIQUE(provider, provider_user_id)
		)
	`)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(
		`INSERT OR IGNORE INTO user (id, username, avatar_url)
		 VALUES (1, 'admin', '')`,
	)
	return err
}

func (s *userStore) FindByProvider(provider, providerUserID string) (*User, error) {
	var user User
	err := s.db.Get(&user,
		`SELECT u.* FROM user u
		 INNER JOIN user_auth a ON a.user_id = u.id
		 WHERE a.provider = ? AND a.provider_user_id = ?`,
		provider, providerUserID)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (s *userStore) CreateUser(username, avatarURL string) (*User, error) {
	result, err := s.db.Exec(
		"INSERT INTO user (username, avatar_url) VALUES (?, ?)",
		username, avatarURL)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	return &User{
		ID:        id,
		Username:  username,
		AvatarURL: avatarURL,
	}, nil
}

func (s *userStore) LinkAuth(userID int64, provider, providerUserID string) error {
	_, err := s.db.Exec(
		"INSERT INTO user_auth (user_id, provider, provider_user_id) VALUES (?, ?, ?)",
		userID, provider, providerUserID)
	if err != nil {
		return fmt.Errorf("link auth: %w", err)
	}
	return nil
}

func (s *userStore) FindByID(id int64) (*User, error) {
	var user User
	err := s.db.Get(&user, "SELECT * FROM user WHERE id = ?", id)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

type providerUserInfo struct {
	Provider       string
	ProviderUserID string
	Username       string
	AvatarURL      string
}

func fetchProviderUserInfo(rm RepositoryManager, token *scm.Token) (*providerUserInfo, error) {
	rmURL := rm.URL().String()

	switch {
	case strings.Contains(rmURL, "github.com"):
		return fetchGitHubUserInfo(token.Token)
	case strings.Contains(rmURL, "gitea"):
		return fetchGiteaUserInfo(rm.URL().String(), token.Token)
	default:
		return fetchGenericUserInfo(rm.URL().String(), token.Token)
	}
}

type rawUserResponse struct {
	ID        int    `json:"id"`
	Login     string `json:"login"`
	Username  string `json:"username"`
	AvatarURL string `json:"avatar_url"`
}

func fetchRawUser(apiURL, accessToken string) (*rawUserResponse, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var ru rawUserResponse
	if err := json.Unmarshal(body, &ru); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return &ru, nil
}

func fetchGitHubUserInfo(accessToken string) (*providerUserInfo, error) {
	ru, err := fetchRawUser("https://api.github.com/user", accessToken)
	if err != nil {
		return nil, err
	}

	username := ru.Login
	if username == "" {
		username = ru.Username
	}

	return &providerUserInfo{
		Provider:       "github",
		ProviderUserID: fmt.Sprintf("%d", ru.ID),
		Username:       username,
		AvatarURL:      ru.AvatarURL,
	}, nil
}

func fetchGiteaUserInfo(baseURL, accessToken string) (*providerUserInfo, error) {
	apiURL := strings.TrimSuffix(baseURL, "/") + "/api/v1/user"
	ru, err := fetchRawUser(apiURL, accessToken)
	if err != nil {
		return nil, err
	}

	username := ru.Username
	if username == "" {
		username = ru.Login
	}
	if username == "" {
		return nil, fmt.Errorf("no username in Gitea user response")
	}

	return &providerUserInfo{
		Provider:       "gitea",
		ProviderUserID: fmt.Sprintf("%d", ru.ID),
		Username:       username,
		AvatarURL:      ru.AvatarURL,
	}, nil
}

func fetchGenericUserInfo(baseURL, accessToken string) (*providerUserInfo, error) {
	apiURL := strings.TrimSuffix(baseURL, "/") + "/api/v1/user"
	ru, err := fetchRawUser(apiURL, accessToken)
	if err != nil {
		return nil, err
	}

	username := ru.Username
	if username == "" {
		username = ru.Login
	}
	if username == "" {
		return nil, fmt.Errorf("no username in user response")
	}

	return &providerUserInfo{
		Provider:       baseURL,
		ProviderUserID: fmt.Sprintf("%d", ru.ID),
		Username:       username,
		AvatarURL:      ru.AvatarURL,
	}, nil
}

func createUserForSession(sess *MoraSession, rm RepositoryManager, token *scm.Token, userStore UserStore) {
	info, err := fetchProviderUserInfo(rm, token)
	if err != nil {
		log.Error().Err(err).Msg("failed to fetch provider user info")
		return
	}

	user, err := userStore.FindByProvider(info.Provider, info.ProviderUserID)
	if err != nil {
		if err == sql.ErrNoRows {
			sess.SetPendingSignup(&pendingSignup{
				rmID:           rm.ID(),
				provider:       info.Provider,
				providerUserID: info.ProviderUserID,
				username:       info.Username,
				avatarURL:      info.AvatarURL,
			})
			return
		}
		log.Error().Err(err).Msg("failed to find user by provider")
		return
	}

	sess.SetUserID(user.ID)
}
