// Package memory is the in-memory implementation of repo.UserRepo. It exists so
// the scaffold runs with zero infrastructure; replace it with a database-backed
// package when real storage is needed.
package memory

import (
	"context"
	"sort"
	"sync"
	"time"

	"simple/internal/repo"
)

// Store is safe for concurrent use. Logic only sees repo.UserRepo, so nothing
// outside this package depends on the concrete type.
type Store struct {
	mu     sync.RWMutex
	nextID int64
	users  map[int64]repo.User
}

var _ repo.UserRepo = (*Store)(nil)

func NewStore() *Store {
	return &Store{users: make(map[int64]repo.User)}
}

func (s *Store) Create(_ context.Context, name, email string) (*repo.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, u := range s.users {
		if u.Email == email {
			return nil, repo.ErrEmailExists
		}
	}

	s.nextID++
	u := repo.User{ID: s.nextID, Name: name, Email: email, CreatedAt: time.Now()}
	s.users[u.ID] = u
	return &u, nil
}

func (s *Store) Get(_ context.Context, id int64) (*repo.User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	u, ok := s.users[id]
	if !ok {
		return nil, repo.ErrNotFound
	}
	return &u, nil
}

func (s *Store) List(_ context.Context, offset, limit int) ([]repo.User, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make([]repo.User, 0, len(s.users))
	for _, u := range s.users {
		users = append(users, u)
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })

	total := len(users)
	if offset < 0 || offset >= total {
		return []repo.User{}, total, nil
	}
	end := min(offset+limit, total)
	return users[offset:end], total, nil
}

func (s *Store) Update(_ context.Context, id int64, name, email string) (*repo.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	u, ok := s.users[id]
	if !ok {
		return nil, repo.ErrNotFound
	}
	for otherID, other := range s.users {
		if otherID != id && other.Email == email {
			return nil, repo.ErrEmailExists
		}
	}
	u.Name = name
	u.Email = email
	s.users[id] = u
	return &u, nil
}

func (s *Store) Delete(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.users[id]; !ok {
		return repo.ErrNotFound
	}
	delete(s.users, id)
	return nil
}
