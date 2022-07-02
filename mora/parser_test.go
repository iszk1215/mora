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
SF:/home/mora/repo/test2.cc
DA:3,1
DA:4,0
end_of_record
`
	buf := bytes.NewBufferString(text)

	prefix := "/home/mora/repo/"
	profiles, err := ParseToolCoverage(buf, "lcov", prefix)

	require.NoError(t, err)
	require.Equal(t, 2, len(profiles))

	require.Equal(t, "test1.cc", profiles[0].FileName)

	blocks := [][]int{{5, 6, 1}, {10, 10, 0}}
	require.Equal(t, blocks, profiles[0].Blocks)
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
	profiles, err := ParseToolCoverage(buf, "go", prefix)

	require.NoError(t, err)
	require.Equal(t, 2, len(profiles))
}
