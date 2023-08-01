package utils

import (
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

var (
	excludeProcess = map[string]bool{
		"nginx":      true,
		"nginx.conf": true,
	}
	// users:(("nginx",pid=31486,fd=8),("nginx",pid=31485,fd=8))
	processPattern = regexp.MustCompile(`\("(.*?)",pid=.*?\)`)
)

func GetListenTCPPorts() (map[int]bool, error) {
	//	/ # ss -ntlp
	//	State                           Recv-Q                          Send-Q                                                     Local Address:Port                                                      Peer Address:Port
	//	LISTEN                          0                               128                                                              0.0.0.0:24220                                                          0.0.0.0:*
	//	LISTEN                          0                               1024                                                             0.0.0.0:24224                                                          0.0.0.0:*
	//	LISTEN                          0                               1024                                                           127.0.0.1:24225                                                          0.0.0.0:*
	//	LISTEN                          0                               2048                                                           127.0.0.1:37698                                                          0.0.0.0:*
	//	LISTEN                          0                               2048                                                           127.0.0.1:10248                                                          0.0.0.0:*
	//	LISTEN                          0                               2048                                                           127.0.0.1:10665                                                          0.0.0.0:*
	//	LISTEN                          0                               511                                                              0.0.0.0:80                                                             0.0.0.0:*                              users:(("nginx",pid=31486,fd=8),("nginx",pid=31485,fd=8),("nginx",pid=31484,fd=8),("nginx",pid=31483,fd=8),("nginx",pid=31482,fd=8),("nginx",pid=31481,fd=8),("nginx",pid=31480,fd=8),("nginx",pid=31479,fd=8),("nginx",pid=48,fd=8))
	//	LISTEN                          0                               511                                                              0.0.0.0:1936                                                           0.0.0.0:*                              users:(("nginx",pid=31486,fd=6),("nginx",pid=31485,fd=6),("nginx",pid=31484,fd=6),("nginx",pid=31483,fd=6),("nginx",pid=31482,fd=6),("nginx",pid=31481,fd=6),("nginx",pid=31480,fd=6),("nginx",pid=31479,fd=6),("nginx",pid=48,fd=6))
	//	LISTEN                          0                               128                                                              0.0.0.0:22                                                             0.0.0.0:*
	//	LISTEN                          0                               2048                                                                   *:31705                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:30715                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:30652                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:32736                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:32000                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:32132                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:30279                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:10249                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:31946                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:31274                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:10250                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:31467                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:9100                                                                 *:*
	//	LISTEN                          0                               2048                                                                   *:30924                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:31436                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:31822                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:10255                                                                *:*
	//	LISTEN                          0                               511                                                                 [::]:80                                                                [::]:*                              users:(("nginx",pid=31486,fd=9),("nginx",pid=31485,fd=9),("nginx",pid=31484,fd=9),("nginx",pid=31483,fd=9),("nginx",pid=31482,fd=9),("nginx",pid=31481,fd=9),("nginx",pid=31480,fd=9),("nginx",pid=31479,fd=9),("nginx",pid=48,fd=9))
	//	LISTEN                          0                               511                                                                 [::]:1936                                                              [::]:*                              users:(("nginx",pid=31486,fd=7),("nginx",pid=31485,fd=7),("nginx",pid=31484,fd=7),("nginx",pid=31483,fd=7),("nginx",pid=31482,fd=7),("nginx",pid=31481,fd=7),("nginx",pid=31480,fd=7),("nginx",pid=31479,fd=7),("nginx",pid=48,fd=7))
	//	LISTEN                          0                               2048                                                                   *:10256                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:1937                                                                 *:*                              users:(("alb",pid=19,fd=7))
	//	LISTEN                          0                               2048                                                                   *:30577                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:31635                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:30900                                                                *:*
	//	LISTEN                          0                               2048                                                                   *:30902                                                                *:*
	//	LISTEN                          0                               128                                                                 [::]:22                                                                [::]:*
	raw, err := exec.Command("ss", "-ntlp").CombinedOutput()
	if err != nil {
		return nil, err
	}
	ports := map[int]bool{}
	output := strings.TrimSpace(string(raw))
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		for _, line := range lines {
			if !strings.Contains(line, "LISTEN") {
				continue
			}
			fields := strings.Fields(line)
			rawLocalAddr := fields[3]
			t := strings.Split(rawLocalAddr, ":")
			port, err := strconv.Atoi(t[len(t)-1])
			if err != nil {
				continue
			}
			processName := "-"
			if len(fields) == 6 {
				rawProcess := fields[5]
				t = processPattern.FindStringSubmatch(rawProcess)
				if len(t) >= 2 {
					processName = t[1]
				}
			}
			if !excludeProcess[processName] {
				ports[port] = true
			}
		}
	}
	return ports, nil
}
