package log_analyze

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	strftime "github.com/ncruces/go-strftime"
)

func parseTime(s string) time.Time {
	t, err := time.Parse("02/Jan/2006:15:04:05 -0700", s)
	if err != nil {
		return time.Now()
	}
	return t
}

func parseTimeLocal(s string) time.Time {
	t, err := strftime.Parse("[%d/%b/%Y:%H:%M:%S %z]", s)
	if err != nil {
		return time.Now()
	}
	return t
}

func parseFloat(s string) float64 {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0
	}
	return f
}

func parseFloats(s string) []float64 {
	ret := []float64{}
	for _, f := range strings.Split(s, ",") {
		ret = append(ret, parseFloat(f))
	}
	return ret
}

func token(line string) []string {
	stack := []string{}
	op := map[string]string{"[": "]", "\"": "\""}
	rop := map[string]string{"]": "[", "\"": "\""}
	seg := []string{}
	cur := ""
	for i := 0; i < len(line); i++ {
		c := string(line[i])
		if c == " " && len(stack) == 0 {
			seg = append(seg, cur)
			cur = ""
			continue
		}
		cur += c
		if rop[c] != "" && len(stack) != 0 && stack[len(stack)-1] == rop[c] {
			// fmt.Println("pop", c, rop[c])
			stack = stack[0 : len(stack)-1]
			seg = append(seg, cur)
			cur = ""
			continue
		}
		if op[c] != "" {
			// fmt.Println("push", c, op[c])
			stack = append(stack, c)
			continue
		}
	}
	seg = append(seg, cur)
	ret := []string{}
	for _, s := range seg {
		if strings.TrimSpace(s) == "" {
			continue
		}
		ret = append(ret, s)
		// fmt.Print(s, " .. ")
	}
	return ret
}

func ParseItHarder(line string) (LogEntry, error) {
	ts := token(line)
	cur := 0
	if len(ts) < 8 {
		return LogEntry{}, fmt.Errorf("line %v", line)
	}
	take := func() string {
		if cur >= len(ts) {
			return ""
		}
		ret := ts[cur]
		cur++
		return ret
	}
	time := take()
	remote := take()
	host := take()
	req := take()
	status := take()
	up_status := func() string {
		s := take()
		for {
			if strings.HasSuffix(s, ",") {
				s += take()
				continue
			}
			break
		}
		return s
	}()
	up_addr := func() string {
		s := take()
		for {
			if strings.HasSuffix(s, ",") {
				s += take()
				continue
			}
			break
		}
		return s
	}()
	http_agent := take()
	x_forward := take()
	req_time := take()
	up_res_time := func() string {
		s := take()
		for {
			if strings.HasSuffix(s, ",") {
				s += take()
				continue
			}
			break
		}
		return s
	}()
	e := LogEntry{
		kind:                 "access",
		raw:                  line,
		TimeLocal:            parseTimeLocal(time),
		RemoteAddr:           remote,
		Host:                 host,
		Request:              req,
		Status:               status,
		UpstreamStatus:       strings.Split(up_status, ","),
		UpstreamAddr:         strings.Split(up_addr, ","),
		HttpUserAgent:        http_agent,
		HttpXForwardedFor:    x_forward,
		RequestTime:          parseFloat(req_time),
		UpstreamResponseTime: parseFloats(up_res_time),
	}
	return e, nil
}

// log_format  http_json escape=json
//
//	'{'
//	      '"time": "$time_iso8601",'
//	      '"msec": "$msec",'
//	      '"client_addr": "$remote_addr",'
//	      '"client_port": "$remote_port",'
//	      '"server_addr": "$server_addr",'
//	      '"server_port": "$server_port",'
//	      '"pid": "$pid",'
//	      '"rule_id": "$rule_id",'
//	      '"host": "$host",'
//	      '"request": "$request",'
//	      '"status": "$status",'
//	      '"upstream_status": "$upstream_status",'
//	      '"upstream_addr": "$upstream_addr",'
//	      '"user_agent": "$http_user_agent",'
//	      '"x-forward-for": "$http_x_forwarded_for",'
//	      '"request_time": "$request_time",'
//	      '"upstream_response_time": "$upstream_response_time",'
//	      '"upstream_bytes_received": "$upstream_bytes_received"'
//	'}';
type AccessLogFmt struct {
	Time                  string `json:"time"`
	Msec                  string `json:"msec"`
	ClientAddr            string `json:"client_addr"`
	ClientPort            string `json:"client_port"`
	ServerAddr            string `json:"server_addr"`
	ServerPort            string `json:"server_port"`
	Pid                   string `json:"pid"`
	RuleId                string `json:"rule_id"`
	Host                  string `json:"host"`
	Request               string `json:"request"`
	Status                string `json:"status"`
	UpstreamStatus        string `json:"upstream_status"`
	UpstreamAddr          string `json:"upstream_addr"`
	UserAgent             string `json:"user_agent"`
	XForwardFor           string `json:"x-forward-for"`
	RequestTime           string `json:"request_time"`
	UpstreamResponseTime  string `json:"upstream_response_time"`
	UpstreamBytesReceived string `json:"upstream_bytes_received"`
}

func ParseAccessLog(line string) (LogEntry, error) {
	log := AccessLogFmt{}
	err := json.Unmarshal([]byte(line), &log)
	if err != nil {
		return LogEntry{}, err
	}
	return LogEntry{
		kind:                 "access",
		raw:                  line,
		TimeLocal:            parseTimeIso8601(log.Time),
		RemoteAddr:           log.ClientAddr,
		Host:                 log.Host,
		Request:              log.Request,
		Status:               log.Status,
		UpstreamStatus:       strings.Split(log.UpstreamStatus, ","),
		UpstreamAddr:         strings.Split(log.UpstreamAddr, ","),
		HttpUserAgent:        log.UserAgent,
		HttpXForwardedFor:    log.XForwardFor,
		RequestTime:          parseFloat(log.RequestTime),
		UpstreamResponseTime: parseFloats(log.UpstreamResponseTime),
	}, nil
}

func parseTimeIso8601(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Now()
	}
	return t
}
