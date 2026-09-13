package server

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/drone/go-scm/scm"
	"github.com/iszk1215/mora/render"
	"github.com/rs/zerolog/log"
)

type moraSessionKey int

const (
	sessionMoraSessionKey moraSessionKey = iota
	contextMoraSessionKey moraSessionKey = iota
)

type pendingSignup struct {
	rmID           int64
	provider       string
	providerUserID string
	username       string
	avatarURL      string
}

type MoraSession struct {
	lock sync.Mutex

	reposMap      map[int64]map[int64]bool // [rmID][repoID]
	tokenMap      map[int64]scm.Token      // [rmID]
	timestamp     time.Time
	loggingInto   int64
	userID        *int64
	pendingSignup *pendingSignup
}

func NewMoraSession() *MoraSession {
	return &MoraSession{
		reposMap:    map[int64]map[int64]bool{},
		tokenMap:    map[int64]scm.Token{},
		timestamp:   time.Now(),
		loggingInto: -1,
	}
}

type moraSessionJSON struct {
	ReposMap      map[int64]map[int64]bool `json:"reposMap,omitempty"`
	TokenMap      map[int64]scm.Token      `json:"tokenMap,omitempty"`
	Timestamp     time.Time                `json:"timestamp"`
	LoggingInto   int64                    `json:"loggingInto"`
	UserID        *int64                   `json:"userID,omitempty"`
	PendingSignup *pendingSignupJSON       `json:"pendingSignup,omitempty"`
}

type pendingSignupJSON struct {
	RMID           int64  `json:"rmID"`
	Provider       string `json:"provider"`
	ProviderUserID string `json:"providerUserID"`
	Username       string `json:"username"`
	AvatarURL      string `json:"avatarURL"`
}

// MarshalJSON serializes the session so it can be persisted in a shared
// database. The lock is held while the snapshot is taken.
func (s *MoraSession) MarshalJSON() ([]byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	dto := moraSessionJSON{
		ReposMap:    s.reposMap,
		TokenMap:    s.tokenMap,
		Timestamp:   s.timestamp,
		LoggingInto: s.loggingInto,
		UserID:      s.userID,
	}
	if s.pendingSignup != nil {
		dto.PendingSignup = &pendingSignupJSON{
			RMID:           s.pendingSignup.rmID,
			Provider:       s.pendingSignup.provider,
			ProviderUserID: s.pendingSignup.providerUserID,
			Username:       s.pendingSignup.username,
			AvatarURL:      s.pendingSignup.avatarURL,
		}
	}
	return json.Marshal(dto)
}

// UnmarshalJSON restores a session persisted by MarshalJSON. Maps are
// normalized back to non-nil so subsequent writes do not panic.
func (s *MoraSession) UnmarshalJSON(b []byte) error {
	var dto moraSessionJSON
	if err := json.Unmarshal(b, &dto); err != nil {
		return err
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	s.reposMap = dto.ReposMap
	if s.reposMap == nil {
		s.reposMap = map[int64]map[int64]bool{}
	}
	s.tokenMap = dto.TokenMap
	if s.tokenMap == nil {
		s.tokenMap = map[int64]scm.Token{}
	}
	s.timestamp = dto.Timestamp
	s.loggingInto = dto.LoggingInto
	s.userID = dto.UserID
	if dto.PendingSignup != nil {
		s.pendingSignup = &pendingSignup{
			rmID:           dto.PendingSignup.RMID,
			provider:       dto.PendingSignup.Provider,
			providerUserID: dto.PendingSignup.ProviderUserID,
			username:       dto.PendingSignup.Username,
			avatarURL:      dto.PendingSignup.AvatarURL,
		}
	}
	return nil
}

func (s *MoraSession) getReposCache(rmID int64) map[int64]bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.reposMap[rmID]
}

func (s *MoraSession) setReposCache(rmID int64, repos map[int64]bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.reposMap[rmID] = repos
}

func (s *MoraSession) getToken(rmID int64) (scm.Token, bool) {
	s.lock.Lock()
	defer s.lock.Unlock()
	token, ok := s.tokenMap[rmID]
	return token, ok
}

func (s *MoraSession) setToken(rmID int64, token scm.Token) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.tokenMap[rmID] = token
}

func (s *MoraSession) Remove(rmID int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.tokenMap, rmID)
	delete(s.reposMap, rmID)
}

func (s *MoraSession) WithToken(ctx context.Context, rmID int64) (context.Context, error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	token, ok := s.tokenMap[rmID]
	if !ok {
		return nil, errorTokenNotFound
	}

	return scm.WithContext(ctx, &token), nil
}

func (s *MoraSession) IsLoggedIn() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.userID != nil
}

func (s *MoraSession) SetUserID(id int64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.userID = &id
}

func (s *MoraSession) UserID() *int64 {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.userID
}

func (s *MoraSession) ClearUserID() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.userID = nil
}

func (s *MoraSession) SetPendingSignup(p *pendingSignup) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pendingSignup = p
}

func (s *MoraSession) PendingSignup() *pendingSignup {
	s.lock.Lock()
	defer s.lock.Unlock()
	return s.pendingSignup
}

func (s *MoraSession) ClearPendingSignup() {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.pendingSignup = nil
}

// Session Manager

type MoraSessionManager struct {
	cookiename string
	store      sessionStore
	lifetime   time.Duration
	stopCh     chan struct{}
	// insecureCookie disables the Secure attribute on the session cookie
	// (for development over plain HTTP).
	insecureCookie bool
	// sessionLocks serializes read-modify-write on a single session within
	// this instance so concurrent requests cannot lose each other's updates.
	sessionLocks sync.Map // sid -> *sync.Mutex
}

func NewMoraSessionManager(insecureCookie bool) *MoraSessionManager {
	m := &MoraSessionManager{
		cookiename:     "morasessionid",
		store:          newMemSessionStore(),
		lifetime:       24 * time.Hour,
		stopCh:         make(chan struct{}),
		insecureCookie: insecureCookie,
	}
	go m.periodicGC()
	return m
}

// NewMoraSessionManagerWithStore creates a session manager backed by the
// given session store (e.g. a database store shared across instances).
func NewMoraSessionManagerWithStore(insecureCookie bool, store sessionStore) *MoraSessionManager {
	m := NewMoraSessionManager(insecureCookie)
	m.store = store
	return m
}

func (m *MoraSessionManager) periodicGC() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.GC()
		case <-m.stopCh:
			return
		}
	}
}

func (m *MoraSessionManager) Close() error {
	close(m.stopCh)
	return nil
}

func WithMoraSession(ctx context.Context, sess *MoraSession) context.Context {
	return context.WithValue(ctx, contextMoraSessionKey, sess)
}

func MoraSessionFrom(ctx context.Context) (*MoraSession, bool) {
	sess, ok := ctx.Value(contextMoraSessionKey).(*MoraSession)
	return sess, ok
}

func sessionID() (string, error) {
	b := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return "", fmt.Errorf("sessionID: %w", err)
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func (m *MoraSessionManager) GC() {
	cutoff := time.Now().Add(-m.lifetime)
	if err := m.store.DeleteExpired(cutoff); err != nil {
		log.Err(err).Msg("session GC failed")
		return
	}

	// Prune per-session locks whose sessions no longer exist. Holding the
	// mutex while deleting the entry guarantees no in-flight request still
	// references it (requests hold their lock through the write-back).
	m.sessionLocks.Range(func(key, value any) bool {
		sid := key.(string)
		mu := value.(*sync.Mutex)
		mu.Lock()
		if !m.store.Has(sid) {
			m.sessionLocks.Delete(sid)
		}
		mu.Unlock()
		return true
	})
}

func (m *MoraSessionManager) lockSession(sid string) func() {
	l, _ := m.sessionLocks.LoadOrStore(sid, &sync.Mutex{})
	mu := l.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (m *MoraSessionManager) SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(m.cookiename)

		var sid string
		if err != nil || cookie.Value == "" {
			sid, err = sessionID()
			if err != nil {
				log.Err(err).Msg("failed to generate session ID")
				render.InternalError(w, err)
				return
			}
		} else {
			sid = cookie.Value
		}

		// Serialize concurrent requests on the same session within this
		// instance so the read-modify-write below is atomic.
		unlock := m.lockSession(sid)

		sess, ok := m.store.Get(sid)
		if !ok {
			log.Info().Msgf("SessionMiddleware: create new MoraSession")
			sess = NewMoraSession()
		}
		sess.lock.Lock()
		sess.timestamp = time.Now()
		sess.lock.Unlock()

		// Write the session back once the handler chain has run, so every
		// mutation made by handlers is persisted. The deferred order (LIFO)
		// keeps the per-session lock held until after the write-back.
		defer func() {
			defer unlock()
			if err := m.store.Put(sid, sess); err != nil {
				log.Err(err).Str("sid", sid).Msg("SessionMiddleware: failed to persist session")
			}
		}()

		cookie = &http.Cookie{
			Name:     m.cookiename,
			Value:    sid,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secureCookieAttr(m.insecureCookie, r),
		}

		http.SetCookie(w, cookie)

		r = r.WithContext(WithMoraSession(r.Context(), sess))
		next.ServeHTTP(w, r)
	})
}
