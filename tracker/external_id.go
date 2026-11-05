package tracker

import (
	"database/sql/driver"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const externalIDPrefix = "trk_"

// ExternalID represents a tracker external ID. In-memory form includes prefix.
// DB form stores bare UUID string.
type ExternalID string

func NewExternalID() (ExternalID, error) {
	id, err := uuid.NewV7()
	if err != nil {
		return "", err
	}
	return ExternalID(externalIDPrefix + id.String()), nil
}

func ParseExternalID(s string) (ExternalID, error) {
	if s == "" {
		return "", fmt.Errorf("invalid tracker id")
	}
	if !strings.HasPrefix(s, externalIDPrefix) {
		return "", fmt.Errorf("invalid tracker id")
	}
	raw := strings.TrimPrefix(s, externalIDPrefix)
	id, err := uuid.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid tracker id")
	}
	return ExternalID(externalIDPrefix + id.String()), nil
}

func (e ExternalID) String() string {
	return string(e)
}

func (e ExternalID) Bare() string {
	if e == "" {
		return ""
	}
	return strings.TrimPrefix(string(e), externalIDPrefix)
}

func (e *ExternalID) Scan(src interface{}) error {
	if src == nil {
		*e = ""
		return nil
	}
	var s string
	switch v := src.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return fmt.Errorf("cannot scan %T into ExternalID", src)
	}
	if s == "" {
		*e = ""
		return nil
	}
	// stored as bare UUID in DB
	id, err := uuid.Parse(s)
	if err != nil {
		return err
	}
	*e = ExternalID(externalIDPrefix + id.String())
	return nil
}

func (e ExternalID) Value() (driver.Value, error) {
	if e == "" {
		return "", nil
	}
	return e.Bare(), nil
}
