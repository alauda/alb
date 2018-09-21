package util

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"net/url"
	"sort"
	"strings"
)

// CreateSignature creates signature for string following Aliyun rules
func CreateSignature(stringToSignature, accessKeySecret string) string {
	// Crypto by HMAC-SHA1
	hmacSha1 := hmac.New(sha1.New, []byte(accessKeySecret))
	hmacSha1.Write([]byte(stringToSignature))
	sign := hmacSha1.Sum(nil)

	// Encode to Base64
	base64Sign := base64.StdEncoding.EncodeToString(sign)

	return base64Sign
}

// CreateSignatureForRequest creates signature for query string values
func CreateSignatureForRequest(method string, values *url.Values, endpoint string, accessKeySecret string) string {

	var sortValues string
	keys := make([]string, len(*values))
	i := 0
	for k := range *values {
		keys[i] = k
		i++
	}
	sort.Strings(keys)
	for _, k := range keys {
		sortValues += "&" + k + "=" + values.Get(k)

	}
	stringToSign := strings.ToUpper(method) + strings.TrimPrefix(endpoint, "https://") + "?" + sortValues[1:]

	return CreateSignature(stringToSign, accessKeySecret)
}
