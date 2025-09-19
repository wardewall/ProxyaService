package auth

import (
	"context"
	"errors"
	"sync"
	"time"

	"ProxyaService/internal/storage"
)

type Service struct {
	allowedUserIDs map[int64]struct{}
	validTokens    map[string]struct{}
	authenticated  map[int64]struct{}
	mu             sync.RWMutex
	store          *storage.Store
}

func New(allowed []int64, tokens []string) *Service {
	ids := make(map[int64]struct{}, len(allowed))
	for _, id := range allowed {
		ids[id] = struct{}{}
	}
	ts := make(map[string]struct{}, len(tokens))
	for _, t := range tokens {
		if t != "" {
			ts[t] = struct{}{}
		}
	}
	return &Service{allowedUserIDs: ids, validTokens: ts, authenticated: make(map[int64]struct{})}
}

// AuthorizeUserByID returns true if user is allowed by ID whitelist (or if whitelist empty => open).
func (s *Service) AuthorizeUserByID(userID int64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// If whitelist set, allow if in whitelist
	if len(s.allowedUserIDs) > 0 {
		if _, ok := s.allowedUserIDs[userID]; ok {
			return true
		}
		// If user authenticated via token, allow as well
		if _, ok := s.authenticated[userID]; ok {
			return true
		}
		return false
	}
	// No whitelist: if there are tokens configured, require authentication
	if len(s.validTokens) > 0 {
		_, ok := s.authenticated[userID]
		return ok
	}
	// Open access if no whitelist and no tokens
	return true
}

var ErrInvalidToken = errors.New("invalid token")

// Authenticate stores auth in context if token is valid (stateless simple flow).
func (s *Service) Authenticate(ctx context.Context, token string, userID int64) (context.Context, error) {
	s.mu.RLock()
	_, ok := s.validTokens[token]
	s.mu.RUnlock()
	if !ok {
		// try DB token if available
		if s.store != nil {
			if role, err := s.store.ConsumeToken(ctx, token, userID); err == nil {
				// mark authed and persist role
				s.mu.Lock()
				s.authenticated[userID] = struct{}{}
				s.mu.Unlock()
				_ = s.store.UpsertUser(ctx, storage.User{ID: userID, Role: role, IsAuthed: true, UpdatedAt: time.Now()})
				return ctx, nil
			}
		}
		return ctx, ErrInvalidToken
	}
	// mark user as authenticated (memory) and persist user
	s.mu.Lock()
	s.authenticated[userID] = struct{}{}
	s.mu.Unlock()
	if s.store != nil {
		_ = s.store.UpsertUser(ctx, storage.User{ID: userID, Role: storage.RoleFree, IsAuthed: true, UpdatedAt: time.Now()})
	}
	return ctx, nil
}

func (s *Service) AttachStore(store *storage.Store) { s.store = store }
