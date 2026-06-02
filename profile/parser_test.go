package profile

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseCoverageLcov(t *testing.T) {
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

	profiles, err := ParseCoverage(buf)

	require.NoError(t, err)

	expected := []*Profile{
		{
			FileName: "/home/mora/repo/test1.cc",
			Hits:     2,
			Lines:    3,
			Blocks:   [][]int{{5, 6, 1}, {10, 10, 0}},
		},
		{
			FileName: "/home/mora/repo/test2.cc",
			Hits:     1,
			Lines:    2,
			Blocks:   [][]int{{3, 3, 1}, {4, 4, 0}},
		},
	}

	require.Equal(t, expected, profiles)
}

func TestParseCoverageLcovDuplicatedFile(t *testing.T) {
	text := `TN:
SF:/home/mora/repo/test1.cc
DA:5,1
DA:6,1
DA:10,0
end_of_record
TN:
SF:/home/mora/repo/test1.cc
DA:3,1
DA:4,0
end_of_record
`
	buf := bytes.NewBufferString(text)

	profiles, err := ParseCoverage(buf)

	require.NoError(t, err)

	expected := []*Profile{
		{
			FileName: "/home/mora/repo/test1.cc",
			Hits:     3,
			Lines:    5,
			Blocks:   [][]int{{3, 3, 1}, {4, 4, 0}, {5, 6, 1}, {10, 10, 0}},
		},
	}

	require.Equal(t, expected, profiles)
}

func TestParseCoverageGo(t *testing.T) {
	text := `mode: set
mockscm.com/mockowner/mockrepo/test.go:1.2,5.4 5 1
mockscm.com/mockowner/mockrepo/test.go:10.2,13.4 3 0
mockscm.com/mockowner/mockrepo/test.go:13.2,30.4 7 1
mockscm.com/mockowner/mockrepo/test2.go:1.2,3.4 3 0
`
	buf := bytes.NewBufferString(text)

	profiles, err := ParseCoverage(buf)

	require.NoError(t, err)

	expected := []*Profile{
		{
			FileName: "mockscm.com/mockowner/mockrepo/test.go",
			Hits:     23,
			Lines:    26,
			Blocks:   [][]int{{1, 5, 1}, {10, 12, 0}, {13, 30, 1}},
		},
		{
			FileName: "mockscm.com/mockowner/mockrepo/test2.go",
			Hits:     0,
			Lines:    3,
			Blocks:   [][]int{{1, 3, 0}},
		},
	}

	require.Equal(t, expected, profiles)
}
