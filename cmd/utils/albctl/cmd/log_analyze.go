package cmd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"alauda.io/alb2/utils/log"
	"github.com/go-logr/logr"
	strftime "github.com/ncruces/go-strftime"
	"github.com/samber/lo"
	"github.com/satyrius/gonx"
	"github.com/spf13/cobra"
)

var analyzerCmd = &cobra.Command{
	Use:   "analyze",
	Short: "analyze logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return Analyze{
			l:    log.InitKlogV2(log.LogCfg{}),
			flag: ANALYZE_FLAG,
		}.run()
	},
}

type AnalyzeFlags struct {
	AccessLogPath                string
	TimeFilter                   string
	reqDelayFilter               int
	ShowLog                      bool
	ShowErrorOnly                bool
	ShowHttpFailOnly             bool
	ShowUpstreamLatency          bool
	ShowUpstreamLatencyOverSteps bool
	timeSteps                    int
	UseStdin                     bool
}

var ANALYZE_FLAG = AnalyzeFlags{}

func init() {
	rootCmd.AddCommand(analyzerCmd)
	flags := analyzerCmd.PersistentFlags()
	flags.StringVar(&ANALYZE_FLAG.AccessLogPath, "access_log_path", "", "Path to the access log file")
	flags.StringVar(&ANALYZE_FLAG.TimeFilter, "time-filter", "", "07:09:52-07:10:21")
	flags.BoolVar(&ANALYZE_FLAG.ShowLog, "show-raw", false, "show raw log")
	flags.BoolVar(&ANALYZE_FLAG.ShowErrorOnly, "show-err-only", false, "show only err log")
	flags.BoolVar(&ANALYZE_FLAG.ShowHttpFailOnly, "show-http-fail-only", false, "show only http fail log")
	flags.BoolVar(&ANALYZE_FLAG.UseStdin, "use-stdin", false, "read log from stdin")
	flags.IntVar(&ANALYZE_FLAG.timeSteps, "range-by-time", 30, "range by each x seconds")
	flags.IntVar(&ANALYZE_FLAG.reqDelayFilter, "req-delay", 0, "filter all request time >= x ms, 0 to disable filter")
	flags.BoolVar(&ANALYZE_FLAG.ShowUpstreamLatency, "show-upstream-latency", false, "show request-time response-time group by upstream")
	flags.BoolVar(&ANALYZE_FLAG.ShowUpstreamLatencyOverSteps, "show-upstream-latency-over-steps", false, "show request-time response-time group by upstream over steps")
}

type LogEntry struct {
	ln                   int
	kind                 string // access | error | unknow
	errKind              string
	raw                  string
	TimeLocal            time.Time
	RemoteAddr           string
	Host                 string
	Request              string
	Status               string
	UpstreamStatus       []string
	UpstreamAddr         []string
	HttpUserAgent        string
	HttpXForwardedFor    string
	RequestTime          float64
	UpstreamResponseTime []float64
}

type Analyze struct {
	l    logr.Logger
	flag AnalyzeFlags
}

func (a Analyze) run() error {
	logEntries, err := a.read_logs()
	if err != nil {
		return err
	}
	if a.flag.TimeFilter != "" {
		startTime, endTime, err := parseTimeFilterFlag(a.flag.TimeFilter)
		if err != nil {
			return fmt.Errorf("failed to parse time filter: %w", err)
		}
		logEntries = filterTimeViaHour(logEntries, startTime, endTime)
	}
	if a.flag.reqDelayFilter != 0 {
		logEntries = lo.Filter(logEntries, func(item LogEntry, index int) bool {
			return item.RequestTime*1000 > float64(a.flag.reqDelayFilter)
		})
	}
	if len(logEntries) == 0 {
		return fmt.Errorf("no log entries found")
	}
	from := logEntries[0].TimeLocal
	if a.flag.ShowLog {
		for _, l := range logEntries {
			if a.flag.ShowErrorOnly && l.kind == "error" {
				fmt.Println(l)
				continue
			}
			if a.flag.ShowHttpFailOnly && l.kind == "access" && l.Status != "200" {
				fmt.Println(l)
				continue
			}
			fmt.Println(l)
		}
	}
	gs := lo.PartitionBy(logEntries, func(e LogEntry) int {
		key := int(e.TimeLocal.Sub(from).Seconds()) / a.flag.timeSteps
		return key
	})
	fmt.Println(a.show(logEntries))
	for _, r := range gs {
		fmt.Println(a.show(r))
	}
	if a.flag.ShowUpstreamLatencyOverSteps {
		fmt.Println(a.upstream_latency_over_steps(gs))
	}
	fmt.Printf("%+v\n", a.flag)
	return nil
}

func (a Analyze) upstream_in_steps(gs [][]LogEntry) any {
	msg := ""
	for _, r := range gs {
		stats := stat(r, func(l LogEntry) float64 { return l.RequestTime })
		msg += fmt.Sprintf("%v\n", stats)
	}
	return msg
}

func gap_detect(ls []LogEntry) string {
	pre := ls[0].TimeLocal
	msg := ""
	for _, e := range ls {
		if e.TimeLocal.Sub(pre) > time.Second*2 {
			msg += fmt.Sprintf("gap %v-%v %vms\n", pre, e.TimeLocal, e.TimeLocal.Sub(pre).Milliseconds())
		}
		pre = e.TimeLocal
	}
	return msg
}

func parseTimeFilterFlag(s string) (time.Time, time.Time, error) {
	parts := strings.Split(s, "-")
	if len(parts) != 2 {
		return time.Time{}, time.Time{}, fmt.Errorf("invalid time filter format: expected 'start,end'")
	}
	startTime, err := time.Parse(time.TimeOnly, strings.TrimSpace(parts[0]))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse start time: %w", err)
	}

	endTime, err := time.Parse(time.TimeOnly, strings.TrimSpace(parts[1]))
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("failed to parse end time: %w", err)
	}
	if startTime.After(endTime) {
		return time.Time{}, time.Time{}, fmt.Errorf("start time must be before end time")
	}

	return startTime, endTime, nil
}

func (a Analyze) read_logs() ([]LogEntry, error) {
	access_log_fmt := "[$time_local] $remote_addr  \"$host\" \"$request\" $status $upstream_status $upstream_addr \"$http_user_agent\" \"$http_x_forwarded_for\" $request_time $upstream_response_time $upstream_bytes_received $server_port"
	var logEntries []LogEntry
	var scanner *bufio.Scanner
	if !a.flag.UseStdin {
		file, err := os.Open(ANALYZE_FLAG.AccessLogPath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		scanner = bufio.NewScanner(file)
	} else {
		scanner = bufio.NewScanner(os.Stdin)
	}
	ngx_parse := gonx.NewParser(access_log_fmt)
	var pre_time time.Time
	ln := 0
	for scanner.Scan() {
		ln++
		line := scanner.Text()
		if strings.Contains(line, "[info]") && strings.Contains(line, "closed keepalive connection") {
			continue
		}
		if strings.Contains(line, "lua") && strings.Contains(line, "[info]") {
			continue
		}
		if strings.Contains(line, "error") {
			entry, err := EntryFromErrLog(line)
			if err != nil {
				entry = LogEntry{
					ln:        ln,
					kind:      "unknow",
					TimeLocal: pre_time,
					raw:       line + err.Error(),
				}
				fmt.Println("unknow when parse error log fail", line, err)
			}
			logEntries = append(logEntries, entry)
			pre_time = entry.TimeLocal
			continue
		}
		raw_entry, err := ngx_parse.ParseString(line)
		if err != nil {
			e := ParseItHarder(line)
			e.ln = ln
			logEntries = append(logEntries, e)
			continue
		}
		entry := EntryFromLog(raw_entry)
		entry.ln = ln
		pre_time = entry.TimeLocal
		logEntries = append(logEntries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}
	return logEntries, nil
}

//nolint:errcheck
func timeOnly(t time.Time) time.Time {
	ret, _ := time.Parse(time.TimeOnly, fmt.Sprintf("%02d:%02d:%02d", t.Hour(), t.Minute(), t.Second()))
	return ret
}

func filterTimeViaHour(ls []LogEntry, from time.Time, to time.Time) []LogEntry {
	filtered := make([]LogEntry, 0)
	for _, entry := range ls {
		local := timeOnly(entry.TimeLocal)
		if local.Equal(from) || local.Equal(to) || local.After(from) && local.Before(to) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func EntryFromErrLog(line string) (LogEntry, error) {
	// Parse the timestamp from the error log line
	parts := strings.SplitN(line, " ", 3)
	if len(parts) < 2 {
		return LogEntry{}, fmt.Errorf("invalid log time fmt %v", line)
	}

	timeStr := parts[0] + " " + parts[1]
	timeLocal, err := time.Parse("2006/01/02 15:04:05", timeStr)
	if err != nil {
		return LogEntry{}, fmt.Errorf("invalid log time fmt %v", line)
	}
	e := LogEntry{
		kind:      "error",
		TimeLocal: timeLocal,
		raw:       line,
	}
	pickUpstream := func(line string) string {
		// 2024/10/14 07:10:21 [error] 48#48: *10518086 upstream timed out (110: Operation timed out) while connecting to upstream, client: 158.246.10.93, server: _, request: "POST /mfs/channel/http.do HTTP/1.1", upstream: "http://158.246.9.68:9080/mfs/channel/http.do", host: "158.246.0.58"
		re := regexp.MustCompile(`upstream: "([^"]+)"`)
		match := re.FindStringSubmatch(line)
		if len(match) > 1 {
			return match[1]
		}
		return ""
	}
	if strings.Contains(line, "(110: Operation timed out) while connecting to upstream") {
		e.errKind = "upstream-timeout"
		e.UpstreamAddr = []string{pickUpstream(line)}
	}
	if strings.Contains(line, "connect() failed (111: Connection refused) while connecting to upstream") {
		e.errKind = "upstream-connect-refused"
		e.UpstreamAddr = []string{pickUpstream(line)}
	}
	if strings.Contains(line, "(104: Connection reset by peer) while reading response header from upstream,") {
		e.errKind = "upstream-rest-when-read-header"
		e.UpstreamAddr = []string{pickUpstream(line)}
	}

	return e, nil
}

func EntryFromLog(rec *gonx.Entry) LogEntry {
	entry := LogEntry{
		kind:                 "access",
		TimeLocal:            parseTime(forceField(rec, "time_local")),
		RemoteAddr:           forceField(rec, "remote_addr"),
		Host:                 forceField(rec, "host"),
		Request:              forceField(rec, "request"),
		Status:               forceField(rec, "status"),
		UpstreamStatus:       strings.Split(forceField(rec, "upstream_status"), ","),
		UpstreamAddr:         strings.Split(forceField(rec, "upstream_addr"), ","),
		HttpUserAgent:        forceField(rec, "http_user_agent"),
		HttpXForwardedFor:    forceField(rec, "http_x_forwarded_for"),
		RequestTime:          parseFloat(forceField(rec, "request_time")),
		UpstreamResponseTime: parseFloats(forceField(rec, "upstream_response_time")),
	}
	return entry
}

type UpstreamStat struct {
	From         time.Time
	To           time.Time
	ErrCount     int
	ErrKindMap   map[string]int
	LeftErr      int
	UnknowCount  int
	RetryCount   int
	HttpErrCount int
	MinV         float64
	MaxV         float64
	Avg          float64
	P50          int64
	P75          int64
	P90          int64
	P99          int64
	P999         int64
}

func (s UpstreamStat) to_string() string {
	ret := fmt.Sprintf("from %v to %v ", s.From.Format("20060102/15:04:05"), s.To.Format("20060102/15:04:05"))
	if s.ErrCount != 0 {
		ret += fmt.Sprintf("err %-5v left %-5v parse_unknow %-5v retry %-5v http_fail %-5v ", s.ErrCount, s.LeftErr, s.UnknowCount, s.RetryCount, s.HttpErrCount)
	}
	if len(s.ErrKindMap) != 0 {
		ret += fmt.Sprintf("err_kind %-5v ", s.ErrKindMap) + "\n"
	}
	if s.LeftErr != 0 {
		ret += fmt.Sprintf("left %-5v ", s.LeftErr)
	}
	if s.UnknowCount != 0 {
		ret += fmt.Sprintf("parse_unknow %-5v ", s.UnknowCount)
	}
	if s.RetryCount != 0 {
		ret += fmt.Sprintf("retry %-5v ", s.RetryCount)
	}
	if s.HttpErrCount != 0 {
		ret += fmt.Sprintf("http_fail %-5v ", s.HttpErrCount)
	}
	ret += fmt.Sprintf("min %7.3fms max %8.3fms avg %7.3fms p50 %7dms p75 %7dms p90 %7dms p99 %7dms p999 %7dms", s.MinV*1000, s.MaxV*1000, s.Avg*1000, s.P50, s.P75, s.P90, s.P99, s.P999)
	return ret
}

func stat(ls []LogEntry, t func(l LogEntry) float64) UpstreamStat {
	min_v := t(ls[0])
	max_v := t(ls[0])
	all := 0.0
	ts := []time.Duration{}
	err_count := 0
	unknow_count := 0
	retry_count := 0
	http_err_count := 0
	err_kind_map := map[string]int{}
	for _, e := range ls {
		if e.kind == "error" {
			err_count++
			if e.errKind != "" {
				err_kind_map[e.errKind]++
			}
		}
		if e.kind == "unknow" {
			unknow_count++
		}
		if e.kind != "access" {
			continue
		}
		if len(e.UpstreamAddr) != 1 {
			retry_count++
		}
		if e.Status != "200" {
			http_err_count++
		}
		te := t(e)
		if te < min_v {
			min_v = te
		}
		if te > max_v {
			max_v = te
		}
		all += te
		td := time.Duration(int64(te * float64(time.Second)))
		ts = append(ts, td)
	}
	left_err := err_count
	for _, c := range err_kind_map {
		left_err -= c
	}
	avg := all / float64(len(ls))
	sort.Slice(ts, func(i, j int) bool {
		return ts[i] < ts[j]
	})
	p := func(total int, p float64) int {
		return int(p*float64(total)+0.5) - 1
	}
	p50 := ts[p(len(ts), 0.5)]
	p75 := ts[p(len(ts), 0.75)]
	p90 := ts[p(len(ts), 0.90)]
	p99 := ts[p(len(ts), 0.99)]
	p999 := ts[p(len(ts), 0.999)]

	stats := UpstreamStat{
		From:         ls[0].TimeLocal,
		To:           ls[len(ls)-1].TimeLocal,
		ErrCount:     err_count,
		ErrKindMap:   err_kind_map,
		LeftErr:      left_err,
		UnknowCount:  unknow_count,
		RetryCount:   retry_count,
		HttpErrCount: http_err_count,
		MinV:         min_v,
		MaxV:         max_v,
		Avg:          avg,
		P50:          p50.Milliseconds(),
		P75:          p75.Milliseconds(),
		P90:          p90.Milliseconds(),
		P99:          p99.Milliseconds(),
		P999:         p999.Milliseconds(),
	}

	return stats
}

func (a Analyze) show(ls []LogEntry) string {
	msg := ""
	from := ls[0].TimeLocal
	to := ls[len(ls)-1].TimeLocal
	from_s := fmt.Sprintf("%v", from)
	to_s := fmt.Sprintf("%v", to)
	dur_s := to.Sub(from).Seconds() + 1
	msg += fmt.Sprintf("time from %v to %v dur %vs count all %v qps %v \n", from_s, to_s, dur_s, len(ls), float64(len(ls))/dur_s)
	msg += fmt.Sprintf("stats %v \n", stat(ls, func(l LogEntry) float64 { return l.RequestTime }).to_string())
	msg += fmt.Sprintf("upstream\n %v \n", upstream_count(ls))
	if a.flag.ShowUpstreamLatency {
		msg += fmt.Sprintf("upstream-request-latency \n%v \n", upstream_latency(ls))
	}
	gap := gap_detect(ls)
	if gap != "" {
		msg += fmt.Sprintf("gap %v \n", gap)
	}
	return msg
}

func upstream_latency(ls []LogEntry) string {
	gs := lo.PartitionBy(ls, func(e LogEntry) string {
		return strings.Join(e.UpstreamAddr, ",")
	})
	msg := ""
	for _, r := range gs {
		if len(r) == 0 {
			continue
		}
		msg += fmt.Sprintf("%-20s req %s\n", r[0].UpstreamAddr[0], stat(r, func(l LogEntry) float64 { return l.RequestTime }).to_string())
		msg += fmt.Sprintf("%-20s res %s\n", r[0].UpstreamAddr[0], stat(r, func(l LogEntry) float64 { return l.UpstreamResponseTime[0] }).to_string())
	}
	return msg
}

func (a Analyze) upstream_latency_over_steps(all [][]LogEntry) string {
	msg := fmt.Sprintf("steps %v\n", len(all))
	upstream_stat_map := map[string][]UpstreamStat{}
	for _, log_in_step := range all {
		log_in_step_each_upstream := lo.PartitionBy(log_in_step, func(e LogEntry) string {
			return strings.Join(e.UpstreamAddr, ",")
		})
		for _, upstream_log_in_step := range log_in_step_each_upstream {
			upstream_stat := stat(upstream_log_in_step, func(l LogEntry) float64 { return l.RequestTime })
			addr := upstream_log_in_step[0].UpstreamAddr[0]
			upstream_stat_map[addr] = append(upstream_stat_map[addr], upstream_stat)
		}
	}
	for k, v := range upstream_stat_map {
		msg += fmt.Sprintf("%v\n", k)
		for _, v := range v {
			msg += fmt.Sprintf("  %s\n", v.to_string())
		}
	}
	return msg
}

func upstream_count(ls []LogEntry) string {
	m := lo.Reduce(ls, func(m map[string]int, l LogEntry, index int) map[string]int {
		for _, a := range l.UpstreamAddr {
			m[a]++
		}
		return m
	}, map[string]int{})
	s := ""
	for k, v := range m {
		s += fmt.Sprintf("%v %v\n", k, v)
	}
	return s
}

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
	f, err := strconv.ParseFloat(s, 64)
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

func forceField(rec *gonx.Entry, name string) string {
	ret, err := rec.Field(name)
	if err != nil {
		return err.Error()
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

func ParseItHarder(line string) LogEntry {
	ts := token(line)
	cur := 0
	take := func() string {
		if cur >= len(ts) {
			panic(line)
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
	return e
}
