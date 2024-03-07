package coverage

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
)

func Test_relativePathFromRoot(t *testing.T) {
	fsys := fstest.MapFS{
		"src/test.cc": &fstest.MapFile{},
	}

	got := relativePathFromRoot("/home/mora/test/src/test.cc", fsys)

	assert.Equal(t, "src/test.cc", got)
}
