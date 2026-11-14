package coverage

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestNewCoverageExternalID(t *testing.T) {
	id, err := NewCoverageExternalID()
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(id), coverageExternalIDPrefix))

	raw, err := uuid.Parse(id.Bare())
	require.NoError(t, err)
	require.Equal(t, uuid.Version(7), raw.Version())

	other, err := NewCoverageExternalID()
	require.NoError(t, err)
	require.NotEqual(t, id, other)
}

func TestParseCoverageExternalID(t *testing.T) {
	raw := uuid.New()
	canonical := coverageExternalIDPrefix + raw.String()

	got, err := ParseCoverageExternalID(canonical)
	require.NoError(t, err)
	require.Equal(t, CoverageExternalID(canonical), got)

	upper := coverageExternalIDPrefix + strings.ToUpper(raw.String())
	got, err = ParseCoverageExternalID(upper)
	require.NoError(t, err)
	require.Equal(t, CoverageExternalID(canonical), got)

	for _, s := range []string{
		"",
		"cov_",
		"cov_not-a-uuid",
		raw.String(),
		"trk_" + raw.String(),
	} {
		_, err := ParseCoverageExternalID(s)
		require.Error(t, err, "input %q", s)
	}
}

func TestCoverageExternalIDScan(t *testing.T) {
	raw := uuid.New()
	canonical := CoverageExternalID(coverageExternalIDPrefix + raw.String())

	tests := []struct {
		name    string
		src     any
		want    CoverageExternalID
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
			var got CoverageExternalID
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

func TestCoverageExternalIDValue(t *testing.T) {
	raw := uuid.New()
	v, err := CoverageExternalID(coverageExternalIDPrefix + raw.String()).Value()
	require.NoError(t, err)
	require.Equal(t, raw.String(), v)

	v, err = CoverageExternalID("").Value()
	require.NoError(t, err)
	require.Equal(t, "", v)
}

func TestCoverageExternalIDUnmarshalJSON(t *testing.T) {
	raw := uuid.New()
	canonical := CoverageExternalID(coverageExternalIDPrefix + raw.String())

	var got CoverageExternalID
	require.NoError(t, json.Unmarshal([]byte(`"`+canonical.String()+`"`), &got))
	require.Equal(t, canonical, got)

	require.Error(t, json.Unmarshal([]byte(`"trk_`+raw.String()+`"`), &got))
}
