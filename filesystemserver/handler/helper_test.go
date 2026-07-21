package handler

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test on Windows: / path semantics differ")
	}
	handler, err := NewFilesystemHandler([]string{"/"})
	assert.NoError(t, err)
	assert.True(t, handler.isPathInAllowedDirs("/etc/hostname"))
}
