package tracker

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	externalIDPrefix       = "trk_"
	seriesExternalIDPrefix = "ser_"
	valueExternalIDPrefix  = "val_"
)

// ExternalID represents a tracker external ID. In-memory form includes prefix.
// DB form stores bare UUID string.
type ExternalID string

// SeriesExternalID represents a series external ID. In-memory form includes
// prefix. DB form stores bare UUID string.
type SeriesExternalID string

// ValueExternalID represents a value external ID. In-memory form includes
// prefix. DB form stores bare UUID string.
type ValueExternalID string

func NewExternalID() (ExternalID, error) {
	s, err := newExternalID(externalIDPrefix)
	if err != nil {
		return "", err
	}
	return ExternalID(s), nil
}

func ParseExternalID(s string) (ExternalID, error) {
	parsed, err := parseExternalID(externalIDPrefix, s)
	if err != nil {
		return "", err
	}
	return ExternalID(parsed), nil
}

func (e ExternalID) String() string {
	return string(e)
}

func (e ExternalID) Bare() string {
	return bareExternalID(externalIDPrefix, string(e))
}

func (e *ExternalID) Scan(src interface{}) error {
	s, err := scanExternalID(externalIDPrefix, src)
	if err != nil {
		return err
	}
	*e = ExternalID(s)
	return nil
}

func (e ExternalID) Value() (driver.Value, error) {
	return externalIDValue(externalIDPrefix, string(e))
}

func NewSeriesExternalID() (SeriesExternalID, error) {
	s, err := newExternalID(seriesExternalIDPrefix)
	if err != nil {
		return "", err
	}
	return SeriesExternalID(s), nil
}

func ParseSeriesExternalID(s string) (SeriesExternalID, error) {
	parsed, err := parseExternalID(seriesExternalIDPrefix, s)
	if err != nil {
		return "", err
	}
	return SeriesExternalID(parsed), nil
}

func (e SeriesExternalID) String() string {
	return string(e)
}

func (e SeriesExternalID) Bare() string {
	return bareExternalID(seriesExternalIDPrefix, string(e))
}

func (e *SeriesExternalID) Scan(src interface{}) error {
	s, err := scanExternalID(seriesExternalIDPrefix, src)
	if err != nil {
		return err
	}
	*e = SeriesExternalID(s)
	return nil
}

func (e SeriesExternalID) Value() (driver.Value, error) {
	return externalIDValue(seriesExternalIDPrefix, string(e))
}

func (e *SeriesExternalID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseSeriesExternalID(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

func NewValueExternalID() (ValueExternalID, error) {
	s, err := newExternalID(valueExternalIDPrefix)
	if err != nil {
		return "", err
	}
	return ValueExternalID(s), nil
}

func ParseValueExternalID(s string) (ValueExternalID, error) {
	parsed, err := parseExternalID(valueExternalIDPrefix, s)
	if err != nil {
		return "", err
	}
	return ValueExternalID(parsed), nil
}

func (e ValueExternalID) String() string {
	return string(e)
}

func (e ValueExternalID) Bare() string {
	return bareExternalID(valueExternalIDPrefix, string(e))
}

func (e *ValueExternalID) Scan(src interface{}) error {
	s, err := scanExternalID(valueExternalIDPrefix, src)
	if err != nil {
		return err
	}
	*e = ValueExternalID(s)
	return nil
}

func (e ValueExternalID) Value() (driver.Value, error) {
	return externalIDValue(valueExternalIDPrefix, string(e))
}

func (e *ValueExternalID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	parsed, err := ParseValueExternalID(s)
	if err != nil {
		return err
	}
	*e = parsed
	return nil
}

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

func scanExternalID(prefix string, src interface{}) (string, error) {
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

func externalIDValue(prefix, s string) (driver.Value, error) {
	if s == "" {
		return "", nil
	}
	return bareExternalID(prefix, s), nil
}
