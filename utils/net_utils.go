package utils

import "strings"

func IsIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func IsIPv6(address string) bool {
	return strings.Count(address, ":") >= 2
}

func IsIPv6Link(address string) bool {
	return strings.Count(address, ":") >= 2 && strings.HasPrefix(address, "fe80")
}
