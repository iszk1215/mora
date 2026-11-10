package coverage

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const coverageExternalIDPrefix = "cov_"

// CoverageExternalID represents a coverage external ID. In-memory form
// includes prefix. DB form stores bare UUID string.
type CoverageExternalID string

func NewCoverageExternalID() (CoverageExternalID, error) {
	s, err := newCoverageExternalID(coverageExternalIDPrefix)
	if err != nil {
		return "", err
	}
	return CoverageExternalID(s), nil
}

func ParseCoverageExternalID(s string) (CoverageExternalID, error) {
	parsed, err := parseCoverageExternalID(coverageExternalIDPrefix, s)
	if err != nil {
		return "", err
	}
	return CoverageExternalID(parsed), nil
}

func (e CoverageExternalID) String() string {
	return string(e)
}

func (e CoverageExternalID) Bare() string {
	return bareCoverageExternalID(coverageExternalIDPrefix, string(e))
}

func (e *CoverageExternalID) Scan(src interface{}) error {
	s, err := scanCoverageExternalID(coverageExternalIDPrefix, src)
	if err != nil {
		return err
	}
	*e = CoverageExternalID(s)
	return nil
}

func (e CoverageExternalID) Value() (driver.Value, error) {
	return coverageExternalIDValue(coverageExternalIDPrefix, string(e))
}

func (e *CoverageExternalID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseCoverageExternalID(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

func newCoverageExternalID(prefix string) (string, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return prefix + id.String(), nil
}

func parseCoverageExternalID(prefix, s string) (string, error) {
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

func bareCoverageExternalID(prefix, s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimPrefix(s, prefix)
}

func scanCoverageExternalID(prefix string, src interface{}) (string, error) {
	if src == nil {
		return "", nil
	}
	var s string
	switch v := src.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return "", fmt.Errorf("cannot scan %T into external ID", src)
	}
	if s == "" {
		return "", nil
	}
	id, err := uuid.Parse(s)
	if err != nil {
		return "", err
	}
	return prefix + id.String(), nil
}

func coverageExternalIDValue(prefix, s string) (driver.Value, error) {
	if s == "" {
		return "", nil
	}
	return bareCoverageExternalID(prefix, s), nil
}
