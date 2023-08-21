package utils

import (
	"net/url"
	"strings"
)

func ParseAddressStr(address string) (ip []string, host []string) {
	return ParseAddressList(SplitAndRemoveEmpty(address, ","))
}

func ParseAddressList(addrs []string) (ip []string, host []string) {
	ip = []string{}
	host = []string{}
	for _, addr := range addrs {
		addr = strings.TrimSpace(addr)
		if addr == "" {
			continue
		}
		v4, v6, hostname, err := addressIs(addr)
		if err != nil {
			continue
		}
		if v4 || v6 {
			ip = append(ip, addr)
		}
		if hostname {
			host = append(host, addr)
		}
	}
	return ip, host
}

func addressIs(address string) (ipv4 bool, ipv6 bool, domain bool, err error) {
	if IsValidIPv4(address) {
		return true, false, false, nil
	}
	if IsValidIPv6(address) {
		return false, true, false, nil
	}
	_, err = url.Parse(address)
	if err != nil {
		return false, false, false, err
	}
	return false, false, true, nil
}
