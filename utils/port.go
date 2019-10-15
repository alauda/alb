package utils

import (
	"github.com/cakturk/go-netstat/netstat"
)

var (
	excludeProcess = map[string]bool{
		"nginx": true,
	}
)

func GetListenTCPPorts() ([]uint16, error) {
	// get all TCP sockets using ports
	tabs, err := netstat.TCPSocks(func(s *netstat.SockTabEntry) bool {
		return s.State == netstat.Listen || s.State == netstat.Established
	})
	if err != nil {
		return nil, err
	}
	var rv []uint16
	for _, e := range tabs {
		// in container, we may not get process name outside
		if e.Process != nil {
			//glog.Infof("port: %s %d", e.Process.Name, e.LocalAddr.Port)
			if !excludeProcess[e.Process.Name] {
				rv = append(rv, e.LocalAddr.Port)
			}
		} else {
			rv = append(rv, e.LocalAddr.Port)
		}
	}
	return rv, nil
}
