package cmd

import (
	"fmt"
	"testing"
)

func TestParse(t *testing.T) {
	lines := []string{
		`[14/Oct/2024:07:11:12 +0000] 158.246.10.93 "158.246.0.58" "POST /mfs/channel/http.do HTTP/1.1" 200 504 200 "158.246.9.68:9080,158.246.9.160:9080" "Apache-HttpClient/4.5.7 (Java/1.8.0_322)" "158.219.208.229, 158.246.0.82" 5.035 5.000,0.035`,
		`[14/Oct/2024:07:11:12 +0000] 158.246.10.93 "158.246.0.58" "POST /mfs/channel/http.do HTTP/1.1" 200 504, 200 158.246.9.68:9080, 158.246.9.160:9080 "Apache-HttpClient/4.5.7 (Java/1.8.0_322)" "158.219.208.229, 158.246.0.82" 5.035 5.000, 0.035`,
	}
	for _, l := range lines {
		fmt.Printf("%+v", ParseItHarder(l))
	}
}
