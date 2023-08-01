package utils

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringSplit(t *testing.T) {
	assert.Equal(t, strings.Split("", ","), []string{""})
	assert.Equal(t, SplitAndRemoveEmpty("", ","), []string{})
}
