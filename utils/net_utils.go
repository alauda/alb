package utils

import (
	"net"
	"strings"
)

func IsIPv4(address string) bool {
	return strings.Count(address, ":") < 2
}

func IsValidIPv4(address string) bool {
	if !IsIPv4(address) {
		return false
	}
	return net.ParseIP(address) != nil
}

func IsIPv6(address string) bool {
	return strings.Count(address, ":") >= 2
}

func IsValidIPv6(address string) bool {
	if !IsIPv6(address) {
		return false
	}
	return net.ParseIP(address) != nil
}

func IsIPv6Link(address string) bool {
	return strings.Count(address, ":") >= 2 && strings.HasPrefix(address, "fe80")
}
