package session_test

import (
	"sync"
	"testing"
	"time"

	"osto-cli-auth/internal/session"
)

func TestSessionLifecycle(t *testing.T) {
	store := session.NewStore()

	// 1. Create session with 2 second TTL
	sess := store.Create("user-123", "alice", 2*time.Second)
	if sess.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if sess.Username != "alice" {
		t.Errorf("expected username alice, got %s", sess.Username)
	}

	// 2. Fetch active session
	retrieved, valid := store.Get(sess.Token)
	if !valid || retrieved == nil {
		t.Fatal("expected session to be valid and found")
	}
	if retrieved.UserID != "user-123" {
		t.Errorf("expected user ID user-123, got %s", retrieved.UserID)
	}

	// 3. Wait for TTL to expire
	time.Sleep(2100 * time.Millisecond)

	// 4. Passive expiration
	_, validAfterExpiry := store.Get(sess.Token)
	if validAfterExpiry {
		t.Error("expected expired session to be invalid and purged")
	}
}

func TestSessionExplicitDeletion(t *testing.T) {
	store := session.NewStore()
	sess := store.Create("user-456", "bob", 10*time.Minute)

	store.Delete(sess.Token)

	_, valid := store.Get(sess.Token)
	if valid {
		t.Error("expected deleted session to not exist")
	}
}

func TestSessionConcurrentAccess(t *testing.T) {
	store := session.NewStore()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			s := store.Create("u", "user", 1*time.Minute)
			store.Get(s.Token)
			if idx%2 == 0 {
				store.Delete(s.Token)
			}
		}(i)
	}

	wg.Wait()
}
