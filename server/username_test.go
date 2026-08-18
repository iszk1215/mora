package server

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSanitizeUsername(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"lowercases", "Octocat", "octocat"},
		{"keeps valid chars", "a-z_0", "a-z_0"},
		{"replaces invalid with hyphen", "John.Doe", "john-doe"},
		{"replaces non-ascii", "山田 tarou", "tarou"},
		{"collapses runs of invalid", "a   b", "a-b"},
		{"avoids double hyphen after separator", "a- b", "a-b"},
		{"trims leading and trailing", "-foo_", "foo"},
		{"empty", "", "user"},
		{"only invalid", "!!!@@#", "user"},
		{"truncates long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"truncates and trims", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa-", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, sanitizeUsername(tt.input))
		})
	}
}

func TestValidateUsername(t *testing.T) {
	tests := []struct {
		name     string
		username string
		wantErr  bool
	}{
		{"valid", "octocat", false},
		{"valid with separators", "my-name_1", false},
		{"empty", "", true},
		{"uppercase", "Octocat", true},
		{"space", "my name", true},
		{"non-ascii", "山田", true},
		{"leading separator", "-foo", true},
		{"trailing separator", "foo-", true},
		{"too long", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", true},
		{"reserved admin", "admin", true},
		{"reserved api", "api", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateUsername(tt.username)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestSuggestUsername(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.CreateUser("octocat", "")
	require.NoError(t, err)
	_, err = store.CreateUser("octocat-2", "")
	require.NoError(t, err)

	t.Run("returns sanitized base when free", func(t *testing.T) {
		got, err := suggestUsername(store, "New User")
		require.NoError(t, err)
		require.Equal(t, "new-user", got)
	})

	t.Run("returns suffixed name when base taken", func(t *testing.T) {
		got, err := suggestUsername(store, "octocat")
		require.NoError(t, err)
		require.Equal(t, "octocat-3", got)
	})

	t.Run("skips reserved names", func(t *testing.T) {
		got, err := suggestUsername(store, "admin")
		require.NoError(t, err)
		require.Equal(t, "admin-2", got)
	})
}

func TestIsUniqueConstraintError(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.CreateUser("octocat", "")
	require.NoError(t, err)

	_, err = store.CreateUser("octocat", "")
	require.Error(t, err)
	require.True(t, isUniqueConstraintError(err), "expected unique constraint error, got %v", err)

	require.False(t, isUniqueConstraintError(errors.New("some other error")))
	require.False(t, isUniqueConstraintError(nil))
}

func TestUserStore_CreateUser_DuplicateUsername(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.CreateUser("octocat", "")
	require.NoError(t, err)

	_, err = store.CreateUser("octocat", "")
	require.Error(t, err)

	// case-insensitive uniqueness
	_, err = store.CreateUser("Octocat", "")
	require.Error(t, err)
}

func TestUserStore_FindByUsername_NotCaseSensitive(t *testing.T) {
	store := newTestUserStore(t)

	_, err := store.CreateUser("octocat", "")
	require.NoError(t, err)

	found, err := store.FindByUsername("Octocat")
	require.NoError(t, err)
	require.Equal(t, "octocat", found.Username)
}
