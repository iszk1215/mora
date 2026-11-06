package tracker

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewExternalID(t *testing.T) {
	id, err := NewExternalID()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(id), externalIDPrefix))

	raw, err := uuid.Parse(id.Bare())
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), raw.Version())

	other, err := NewExternalID()
	require.NoError(t, err)
	require.NotEqual(t, id, other)
}

func TestParseExternalID(t *testing.T) {
	raw := uuid.New()
	canonical := externalIDPrefix + raw.String()

	got, err := ParseExternalID(canonical)
	require.NoError(t, err)
	require.Equal(t, ExternalID(canonical), got)

	// Parse normalizes uppercase UUIDs so they match the lowercase DB form.
	upper := externalIDPrefix + strings.ToUpper(raw.String())
	got, err = ParseExternalID(upper)
	require.NoError(t, err)
	require.Equal(t, ExternalID(canonical), got)

	for _, s := range []string{
		"",
		"trk_",
		"trk_not-a-uuid",
		raw.String(),
		"cov_" + raw.String(),
	} {
		_, err := ParseExternalID(s)
		require.Error(t, err, "input %q", s)
	}
}

func TestExternalIDScan(t *testing.T) {
	raw := uuid.New()
	canonical := ExternalID(externalIDPrefix + raw.String())

	tests := []struct {
		name    string
		src     any
		want    ExternalID
		wantErr bool
	}{
		{name: "string", src: raw.String(), want: canonical},
		{name: "bytes", src: []byte(raw.String()), want: canonical},
		{name: "uppercase", src: strings.ToUpper(raw.String()), want: canonical},
		{name: "nil", src: nil, want: ""},
		{name: "empty string", src: "", want: ""},
		{name: "invalid uuid", src: "not-a-uuid", wantErr: true},
		{name: "unsupported type", src: 42, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got ExternalID
			err := got.Scan(tt.src)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestExternalIDValue(t *testing.T) {
	raw := uuid.New()
	v, err := ExternalID(externalIDPrefix + raw.String()).Value()
	require.NoError(t, err)
	require.Equal(t, raw.String(), v)

	v, err = ExternalID("").Value()
	require.NoError(t, err)
	require.Equal(t, "", v)
}

func TestSeriesExternalID(t *testing.T) {
	id, err := NewSeriesExternalID()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(id), seriesExternalIDPrefix))

	raw, err := uuid.Parse(id.Bare())
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), raw.Version())

	parsed, err := ParseSeriesExternalID(strings.ToUpper(string(id)))
	require.NoError(t, err)
	require.Equal(t, id, parsed)

	_, err = ParseSeriesExternalID("trk_" + uuid.NewString())
	require.Error(t, err)
	_, err = ParseSeriesExternalID("ser_not-a-uuid")
	require.Error(t, err)
}

func TestSeriesExternalIDScanValue(t *testing.T) {
	raw := uuid.New()
	canonical := SeriesExternalID(seriesExternalIDPrefix + raw.String())

	var got SeriesExternalID
	require.NoError(t, got.Scan(raw.String()))
	require.Equal(t, canonical, got)

	v, err := canonical.Value()
	require.NoError(t, err)
	require.Equal(t, raw.String(), v)

	var fromJSON SeriesExternalID
	require.NoError(t, json.Unmarshal([]byte(`"`+canonical.String()+`"`), &fromJSON))
	require.Equal(t, canonical, fromJSON)

	err = json.Unmarshal([]byte(`"trk_`+raw.String()+`"`), &fromJSON)
	require.Error(t, err)
}

func TestValueExternalID(t *testing.T) {
	id, err := NewValueExternalID()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(id), valueExternalIDPrefix))

	raw, err := uuid.Parse(id.Bare())
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), raw.Version())

	parsed, err := ParseValueExternalID(strings.ToUpper(string(id)))
	require.NoError(t, err)
	require.Equal(t, id, parsed)

	_, err = ParseValueExternalID("ser_" + uuid.NewString())
	require.Error(t, err)
}

func TestValueExternalIDScanValueUnmarshalJSON(t *testing.T) {
	raw := uuid.New()
	canonical := ValueExternalID(valueExternalIDPrefix + raw.String())

	var got ValueExternalID
	require.NoError(t, got.Scan(raw.String()))
	require.Equal(t, canonical, got)

	v, err := canonical.Value()
	require.NoError(t, err)
	require.Equal(t, raw.String(), v)

	var fromJSON ValueExternalID
	require.NoError(t, json.Unmarshal([]byte(`"`+canonical.String()+`"`), &fromJSON))
	require.Equal(t, canonical, fromJSON)

	err = json.Unmarshal([]byte(`"not-a-value-id"`), &fromJSON)
	require.Error(t, err)
}
