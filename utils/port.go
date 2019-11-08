package utils

import (
	"os/exec"
	"strconv"
	"strings"
)

var (
	excludeProcess = map[string]bool{
		"nginx":      true,
		"nginx.conf": true,
	}
)

func GetListenTCPPorts() ([]int, error) {
	///var/log/mathilde # netstat -ntlp
	//Active Internet connections (only servers)
	//Proto Recv-Q Send-Q Local Address           Foreign Address         State       PID/Program name
	//tcp        0      0 127.0.0.1:25            0.0.0.0:*               LISTEN      -
	//tcp        0      0 127.0.0.1:10248         0.0.0.0:*               LISTEN      -
	//tcp        0      0 127.0.0.1:9099          0.0.0.0:*               LISTEN      -
	//tcp        0      0 0.0.0.0:9997            0.0.0.0:*               LISTEN      46/nginx.conf
	//tcp        0      0 127.0.0.1:35885         0.0.0.0:*               LISTEN      -
	//tcp        0      0 0.0.0.0:9999            0.0.0.0:*               LISTEN      46/nginx.conf
	//tcp        0      0 0.0.0.0:1936            0.0.0.0:*               LISTEN      46/nginx.conf
	//tcp        0      0 0.0.0.0:179             0.0.0.0:*               LISTEN      -
	//tcp        0      0 0.0.0.0:22              0.0.0.0:*               LISTEN      -
	//tcp        0      0 ::1:25                  :::*                    LISTEN      -
	//tcp        0      0 :::10249                :::*                    LISTEN      -
	//tcp        0      0 :::10250                :::*                    LISTEN      -
	//tcp        0      0 :::9997                 :::*                    LISTEN      46/nginx.conf
	//tcp        0      0 :::9999                 :::*                    LISTEN      46/nginx.conf
	//tcp        0      0 :::10255                :::*                    LISTEN      -
	//tcp        0      0 :::1936                 :::*                    LISTEN      46/nginx.conf
	//tcp        0      0 :::10256                :::*                    LISTEN      -
	//tcp        0      0 :::1937                 :::*                    LISTEN      18/alb
	raw, err := exec.Command("netstat", "-ntlp").CombinedOutput()
	if err != nil {
		return nil, err
	}
	var ports []int
	output := strings.TrimSpace(string(raw))
	lines := strings.Split(output, "\n")
	if len(lines) > 0 {
		for _, line := range lines {
			if !strings.Contains(line, "tcp") {
				continue
			}
			fields := strings.Fields(line)
			rawLocalAddr := fields[3]
			t := strings.Split(rawLocalAddr, ":")
			port, _ := strconv.Atoi(t[len(t)-1])
			rawProcess := fields[6]
			processName := "-"
			if strings.Contains(rawProcess, "/") {
				t = strings.Split(rawProcess, "/")
				processName = t[1]
			}
			if !excludeProcess[processName] {
				ports = append(ports, port)
			}
		}
	}
	return ports, nil
}
