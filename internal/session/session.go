package session

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// Session holds in-memory session details.
type Session struct {
	Token     string
	UserID    string
	Username  string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Remaining returns how much time is left before session expiration.
func (s *Session) Remaining(now time.Time) time.Duration {
	if now.After(s.ExpiresAt) {
		return 0
	}
	return s.ExpiresAt.Sub(now).Round(time.Second)
}

// Store manages in-memory sessions with concurrent safety.
type Store struct {
	mu       sync.RWMutex
	sessions map[string]*Session
}

// NewStore creates an empty session store.
func NewStore() *Store {
	return &Store{
		sessions: make(map[string]*Session),
	}
}

// Create creates and stores a new session with the given TTL.
func (s *Store) Create(userID, username string, ttl time.Duration) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	sess := &Session{
		Token:     uuid.NewString(),
		UserID:    userID,
		Username:  username,
		CreatedAt: now,
		ExpiresAt: now.Add(ttl),
	}
	s.sessions[sess.Token] = sess
	return sess
}

// Get retrieves a session by token. If expired, it passively deletes the session and returns false.
func (s *Store) Get(token string) (*Session, bool) {
	s.mu.RLock()
	sess, exists := s.sessions[token]
	s.mu.RUnlock()

	if !exists {
		return nil, false
	}

	if time.Now().UTC().After(sess.ExpiresAt) {
		s.Delete(token)
		return nil, false
	}

	return sess, true
}

// Delete removes a session token.
func (s *Store) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

// DeleteByUserID removes all sessions associated with a specific user ID.
func (s *Store) DeleteByUserID(userID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for token, sess := range s.sessions {
		if sess.UserID == userID {
			delete(s.sessions, token)
		}
	}
}
