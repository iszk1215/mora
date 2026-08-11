package server

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/mattn/go-sqlite3"
)

// maxUsernameLength is the maximum length of a username.
const maxUsernameLength = 32

// reservedUsernames are names that cannot be claimed as a username. They are
// reserved for future use as URL path segments (e.g. /login, /api/me).
var reservedUsernames = map[string]struct{}{
	"admin":     {},
	"api":       {},
	"auth":      {},
	"coverages": {},
	"login":     {},
	"logout":    {},
	"me":        {},
	"repos":     {},
	"settings":  {},
	"signup":    {},
	"swagger":   {},
	"trackers":  {},
	"users":     {},
}

// isUsernameChar reports whether r is allowed inside a username: lowercase
// ASCII letters, digits, '-' or '_'.
func isUsernameChar(r rune) bool {
	return isUsernameAlphanumeric(r) ||
		r == '-' || r == '_'
}

// isUsernameAlphanumeric reports whether r is a lowercase ASCII letter or digit.
func isUsernameAlphanumeric(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= '0' && r <= '9'
}

// sanitizeUsername converts an arbitrary input (such as a provider username)
// into a valid URL-safe username candidate: it lowercases, replaces runs of
// disallowed characters with a single '-', trims leading/trailing '-'/'_' and
// truncates to maxUsernameLength. An empty result falls back to "user".
func sanitizeUsername(input string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(input) {
		if isUsernameChar(r) {
			b.WriteRune(r)
		} else if b.Len() == 0 || b.String()[b.Len()-1] != '-' {
			b.WriteByte('-')
		}
	}

	name := strings.Trim(b.String(), "-_")
	if name == "" {
		return "user"
	}
	if len(name) > maxUsernameLength {
		name = strings.TrimRight(name[:maxUsernameLength], "-_")
		if name == "" {
			return "user"
		}
	}
	return name
}

// validateUsername checks that a username follows the username rules: lowercase
// letters, digits, '-' or '_', 1..maxUsernameLength characters, an alphanumeric
// leading/trailing character and not a reserved name.
func validateUsername(username string) error {
	if username == "" {
		return errors.New("username is required")
	}
	if username != strings.ToLower(username) {
		return errors.New("username must be lowercase")
	}
	if len(username) > maxUsernameLength {
		return fmt.Errorf("username must be at most %d characters", maxUsernameLength)
	}
	if !isUsernameAlphanumeric(rune(username[0])) || !isUsernameAlphanumeric(rune(username[len(username)-1])) {
		return errors.New("username must start and end with a letter or digit")
	}
	for _, r := range username {
		if !isUsernameChar(r) {
			return errors.New("username may only contain lowercase letters, digits, '-' and '_'")
		}
	}
	if _, reserved := reservedUsernames[username]; reserved {
		return fmt.Errorf("username %q is reserved", username)
	}
	return nil
}

// suggestUsername returns an available username derived from base. It returns
// the sanitized base when free, otherwise base-2, base-3, ... skipping reserved
// names and usernames already taken by other users.
func suggestUsername(userStore UserStore, base string) (string, error) {
	root := sanitizeUsername(base)
	if len(root) > maxUsernameLength-4 {
		root = strings.TrimRight(root[:maxUsernameLength-4], "-_")
		if root == "" {
			root = "user"
		}
	}

	for i := 0; ; i++ {
		candidate := root
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", root, i+1)
		}
		if _, reserved := reservedUsernames[candidate]; reserved {
			continue
		}
		_, err := userStore.FindByUsername(candidate)
		if err == sql.ErrNoRows {
			return candidate, nil
		}
		if err != nil {
			return "", fmt.Errorf("suggestUsername: %w", err)
		}
	}
}

// isUniqueConstraintError reports whether err is a SQLite UNIQUE constraint
// violation (e.g. inserting a duplicate username).
func isUniqueConstraintError(err error) bool {
	var sqliteErr sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code == sqlite3.ErrConstraint &&
			sqliteErr.ExtendedCode == sqlite3.ErrConstraintUnique
	}
	return false
}
