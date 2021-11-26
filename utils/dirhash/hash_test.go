// Copy from https://github.com/golang/mod/blob/master/sumdb/dirhash/hash.go
package dirhash

import (
	"crypto/sha256"
	"encoding/base32"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestLabelSafeHash(t *testing.T) {
	origin := "a" + strings.Repeat("b", 1000)
	h := sha256.New()
	h.Write([]byte(origin))
	sha256Result := h.Sum(nil)
	str := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sha256Result)
	t.Logf("str %s %v", str, len(str))
	assert.Equal(t, true, len(str) == 52)

}
