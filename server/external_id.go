package server

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	repositoryExternalIDPrefix = "repo_"
	scmExternalIDPrefix        = "scm_"
	apiKeyExternalIDPrefix     = "key_"
)

func newExternalID(prefix string) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return prefix + id.String(), nil
}

func parseExternalID(prefix, s string) (string, error) {
	if s == "" || len(s) < len(prefix) || !strings.EqualFold(s[:len(prefix)], prefix) {
		return "", fmt.Errorf("invalid external id")
	}
	raw := s[len(prefix):]
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid external id")
	}
	return prefix + id.String(), nil
}

func bareExternalID(prefix, s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimPrefix(s, prefix)
}

func prefixExternalID(prefix, bare string) string {
	if bare == "" {
		return ""
	}
	return prefix + bare
}

func newRepositoryExternalID() (string, error) {
	return newExternalID(repositoryExternalIDPrefix)
}

func parseRepositoryExternalID(s string) (string, error) {
	return parseExternalID(repositoryExternalIDPrefix, s)
}

func bareRepositoryExternalID(s string) string {
	return bareExternalID(repositoryExternalIDPrefix, s)
}

func repositoryExternalIDFromBare(bare string) string {
	return prefixExternalID(repositoryExternalIDPrefix, bare)
}

func newSCMExternalID() (string, error) {
	return newExternalID(scmExternalIDPrefix)
}

func parseSCMExternalID(s string) (string, error) {
	return parseExternalID(scmExternalIDPrefix, s)
}

func scmExternalIDFromBare(bare string) string {
	return prefixExternalID(scmExternalIDPrefix, bare)
}

func newAPIKeyExternalID() (string, error) {
	return newExternalID(apiKeyExternalIDPrefix)
}

func parseAPIKeyExternalID(s string) (string, error) {
	return parseExternalID(apiKeyExternalIDPrefix, s)
}

func bareAPIKeyExternalID(s string) string {
	return bareExternalID(apiKeyExternalIDPrefix, s)
}

func apiKeyExternalIDFromBare(bare string) string {
	return prefixExternalID(apiKeyExternalIDPrefix, bare)
}
