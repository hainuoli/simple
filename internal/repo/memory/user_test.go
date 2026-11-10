package memory

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"simple/internal/repo"
)

func TestCreateAndGet(t *testing.T) {
	ctx := context.Background()
	s := NewStore()

	created, err := s.Create(ctx, "alice", "alice@example.com")
	require.NoError(t, err)
	require.NotZero(t, created.ID)
	require.False(t, created.CreatedAt.IsZero())

	got, err := s.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, *created, *got)
}

func TestCreateRejectsDuplicateEmail(t *testing.T) {
	ctx := context.Background()
	s := NewStore()

	_, err := s.Create(ctx, "alice", "alice@example.com")
	require.NoError(t, err)

	_, err = s.Create(ctx, "bob", "alice@example.com")
	require.ErrorIs(t, err, repo.ErrEmailExists)
}

func TestGetMissing(t *testing.T) {
	_, err := NewStore().Get(context.Background(), 1)
	require.ErrorIs(t, err, repo.ErrNotFound)
}

func TestListPagination(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	for _, name := range []string{"a", "b", "c"} {
		_, err := s.Create(ctx, name, name+"@example.com")
		require.NoError(t, err)
	}

	page, total, err := s.List(ctx, 1, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, page, 1)
	require.Equal(t, "b", page[0].Name)

	empty, total, err := s.List(ctx, 10, 1)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.NotNil(t, empty)
	require.Empty(t, empty)
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	created, err := s.Create(ctx, "alice", "alice@example.com")
	require.NoError(t, err)
	_, err = s.Create(ctx, "bob", "bob@example.com")
	require.NoError(t, err)

	updated, err := s.Update(ctx, created.ID, "alice2", "alice2@example.com")
	require.NoError(t, err)
	require.Equal(t, "alice2", updated.Name)
	require.Equal(t, "alice2@example.com", updated.Email)
}

func TestUpdateErrors(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	created, err := s.Create(ctx, "alice", "alice@example.com")
	require.NoError(t, err)
	_, err = s.Create(ctx, "bob", "bob@example.com")
	require.NoError(t, err)

	_, err = s.Update(ctx, created.ID, "alice", "bob@example.com")
	require.ErrorIs(t, err, repo.ErrEmailExists)

	_, err = s.Update(ctx, 999, "ghost", "ghost@example.com")
	require.ErrorIs(t, err, repo.ErrNotFound)
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	s := NewStore()
	created, err := s.Create(ctx, "alice", "alice@example.com")
	require.NoError(t, err)

	require.NoError(t, s.Delete(ctx, created.ID))

	_, err = s.Get(ctx, created.ID)
	require.ErrorIs(t, err, repo.ErrNotFound)

	require.ErrorIs(t, s.Delete(ctx, created.ID), repo.ErrNotFound)
}
