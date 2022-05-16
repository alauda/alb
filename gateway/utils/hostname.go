package utils

import "strings"

func FindIntersection(lsHost string, routeHost []string) []string {
	if len(routeHost) == 0 {
		return []string{lsHost}
	}
	ret := []string{}

	for _, rh := range routeHost {
		if matchDomain(lsHost, rh) {
			ret = append(ret, rh)
		}
	}
	return ret
}

func matchDomain(host, route string) bool {
	hostIsWildcard := strings.HasPrefix(host, "*.")
	routeIsWildcard := strings.HasPrefix(route, "*.")
	if hostIsWildcard && !routeIsWildcard {
		hostWithoutWildcard := strings.TrimPrefix(host, "*")
		return strings.HasSuffix(route, hostWithoutWildcard)
	}
	if !hostIsWildcard && routeIsWildcard {
		routeWithoutWildcard := strings.TrimPrefix(route, "*")
		return strings.HasSuffix(host, routeWithoutWildcard)
	}
	return host == route
}
