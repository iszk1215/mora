package mora

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseToolCoverageLcov(t *testing.T) {
	text := `TN:
SF:/home/mora/repo/test1.cc
DA:5,1
DA:6,1
DA:10,0
end_of_record
TN:
SF:/home/mora/repo/test2.cc
DA:3,1
DA:4,0
end_of_record
`
	buf := bytes.NewBufferString(text)

	prefix := "/home/mora/repo"
	profiles, err := ParseCoverage(buf, "lcov", prefix)

	require.NoError(t, err)

	expected := []*Profile{
		{
			FileName: "test1.cc",
			Hits:     2,
			Lines:    3,
			Blocks:   [][]int{{5, 6, 1}, {10, 10, 0}},
		},
		{
			FileName: "test2.cc",
			Hits:     1,
			Lines:    2,
			Blocks:   [][]int{{3, 3, 1}, {4, 4, 0}},
		},
	}

	require.Equal(t, expected, profiles)
}

func TestParseToolCoverageGo(t *testing.T) {
	text := `mode: set
mockscm.com/mockowner/mockrepo/test.go:1.2,5.4 5 1
mockscm.com/mockowner/mockrepo/test.go:10.2,13.4 3 0
mockscm.com/mockowner/mockrepo/test.go:13.2,30.4 7 1
mockscm.com/mockowner/mockrepo/test2.go:1.2,3.4 3 0
`
	buf := bytes.NewBufferString(text)

	prefix := "mockscm.com/mockowner/mockrepo"
	profiles, err := ParseCoverage(buf, "go", prefix)

	require.NoError(t, err)

	expected := []*Profile{
		{
			FileName: "test.go",
			Hits:     23,
			Lines:    26,
			Blocks:   [][]int{{1, 5, 1}, {10, 12, 0}, {13, 30, 1}},
		},
		{
			FileName: "test2.go",
			Hits:     0,
			Lines:    3,
			Blocks:   [][]int{{1, 3, 0}},
		},
	}

	require.Equal(t, expected, profiles)
}
