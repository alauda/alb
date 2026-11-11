package config

import (
	"fmt"
	"testing"
)

func TestCheckIPV6Supported(t *testing.T) {
	ipv6Supported = checkIPV6Supported()
	fmt.Println(ipv6Supported)
}
