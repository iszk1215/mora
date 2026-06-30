package server

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/hex"
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

type UserAPIKey struct {
	ID        int64  `db:"id" json:"id"`
	UserID    int64  `db:"user_id" json:"-"`
	Name      string `db:"name" json:"name"`
	KeyHash   string `db:"key_hash" json:"-"`
	KeyPrefix string `db:"key_prefix" json:"key_prefix"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

type UserStore interface {
	Init() error
	FindByProvider(provider, providerUserID string) (*User, error)
	CreateUser(username, avatarURL string) (*User, error)
	LinkAuth(userID int64, provider, providerUserID string) error
	FindByID(id int64) (*User, error)
	CreateAPIKey(userID int64, name string) (string, error)
	ListAPIKeys(userID int64) ([]UserAPIKey, error)
	RevokeAPIKey(userID, keyID int64) error
	FindUserByAPIKey(plaintext string) (*User, error)
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

	_, err = s.db.Exec(`
		CREATE TABLE IF NOT EXISTS user_api_key (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id    INTEGER NOT NULL REFERENCES user(id),
			name       TEXT    NOT NULL,
			key_hash   TEXT    NOT NULL,
			key_prefix TEXT    NOT NULL,
			created_at TEXT    NOT NULL DEFAULT (datetime('now'))
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

func generateAPIKey() (string, string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", "", fmt.Errorf("rand.Read: %w", err)
	}
	plaintext := "mora_" + hex.EncodeToString(b)
	hash := sha256.Sum256([]byte(plaintext))
	return plaintext, hex.EncodeToString(hash[:]), nil
}

func (s *userStore) CreateAPIKey(userID int64, name string) (string, error) {
	plaintext, keyHash, err := generateAPIKey()
	if err != nil {
		return "", fmt.Errorf("CreateAPIKey generate: %w", err)
	}
	prefix := plaintext[:12]
	_, err = s.db.Exec(
		"INSERT INTO user_api_key (user_id, name, key_hash, key_prefix) VALUES (?, ?, ?, ?)",
		userID, name, keyHash, prefix)
	if err != nil {
		return "", fmt.Errorf("CreateAPIKey insert: %w", err)
	}
	return plaintext, nil
}

func (s *userStore) ListAPIKeys(userID int64) ([]UserAPIKey, error) {
	var keys []UserAPIKey
	err := s.db.Select(&keys,
		"SELECT id, user_id, name, key_prefix, created_at FROM user_api_key WHERE user_id = ? ORDER BY created_at DESC",
		userID)
	if err != nil {
		return nil, fmt.Errorf("ListAPIKeys: %w", err)
	}
	return keys, nil
}

func (s *userStore) RevokeAPIKey(userID, keyID int64) error {
	result, err := s.db.Exec(
		"DELETE FROM user_api_key WHERE id = ? AND user_id = ?",
		keyID, userID)
	if err != nil {
		return fmt.Errorf("RevokeAPIKey: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("RevokeAPIKey RowsAffected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("api key not found")
	}
	return nil
}

func (s *userStore) FindUserByAPIKey(plaintext string) (*User, error) {
	hash := sha256.Sum256([]byte(plaintext))
	keyHash := hex.EncodeToString(hash[:])

	var ak UserAPIKey
	err := s.db.Get(&ak,
		"SELECT user_id, key_hash FROM user_api_key WHERE key_hash = ?", keyHash)
	if err != nil {
		return nil, fmt.Errorf("FindUserByAPIKey: %w", err)
	}

	// Constant-time verification: decode both hashes and compare
	computed, _ := hex.DecodeString(keyHash)
	stored, _ := hex.DecodeString(ak.KeyHash)
	if subtle.ConstantTimeCompare(computed, stored) != 1 {
		return nil, fmt.Errorf("api key hash mismatch")
	}

	return s.FindByID(ak.UserID)
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
