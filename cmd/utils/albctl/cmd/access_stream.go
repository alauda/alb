package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"

	"alauda.io/alb2/utils/log"

	"github.com/bcicen/jstream"
	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
)

type Metric struct {
	Name           string `json:"__name__"`
	Job            string `json:"job"`
	XId            string `json:"x_id"` // make sure each metric has a unique x_id
	ClientAddr     string `json:"client_addr"`
	ClientPort     string `json:"client_port"`
	ServerAddr     string `json:"server_addr"`
	ServerPort     string `json:"server_port"`
	PID            string `json:"pid"`
	RuleID         string `json:"rule_id"`
	Host           string `json:"host"`
	Request        string `json:"request"`
	Status         string `json:"status"`
	UpstreamStatus string `json:"upstream_status"`
	UpstreamAddr   string `json:"upstream_addr"`
	UserAgent      string `json:"user_agent"`
	XForwardFor    string `json:"x-forward-for"`
}

// the json line of alb access log which we want send to victoria-metrics
type VmAccess struct {
	Metric     Metric    `json:"metric"`
	Values     []float64 `json:"values"`
	Timestamps []int64   `json:"timestamps"`
}

var LogStreamCmd = &cobra.Command{
	Use:   "log-stream",
	Short: "stream log to victoria-metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return LogStream{
			l:    log.InitKlogV2(log.LogCfg{}),
			flag: LOGSTREAM_FLAG,
		}.run()
	},
}

type LogStreamFlags struct {
	UseStdin   bool
	InputFile  string
	OutputFile string
}

var LOGSTREAM_FLAG = LogStreamFlags{}

func init() {
	rootCmd.AddCommand(LogStreamCmd)
	flags := LogStreamCmd.PersistentFlags()
	flags.StringVar(&LOGSTREAM_FLAG.InputFile, "input-file", "", "Path to the access log file")
	flags.StringVar(&LOGSTREAM_FLAG.OutputFile, "output-file", "", "Path to the output file")
	flags.BoolVar(&LOGSTREAM_FLAG.UseStdin, "use-stdin", false, "read log from stdin")
}

type LogStream struct {
	l    logr.Logger
	flag LogStreamFlags
}

func (s LogStream) run() error {
	if s.flag.InputFile == "" && !s.flag.UseStdin {
		return fmt.Errorf("input-file is required")
	}
	if s.flag.OutputFile == "" {
		return fmt.Errorf("output-file is required")
	}
	var r io.Reader
	if s.flag.InputFile == "stdin" {
		r = os.Stdin
	} else {
		rf, err := os.Open(s.flag.InputFile)
		if err != nil {
			return fmt.Errorf("Failed to open input file: %v", err)
		}
		r = rf
	}
	wf, err := os.Create(s.flag.OutputFile)
	if err != nil {
		return fmt.Errorf("Failed to open output file: %v", err)
	}
	w := bufio.NewWriter(wf)
	if err := s.trans(r, w); err != nil {
		return fmt.Errorf("Failed to trans: %v", err)
	}
	w.Flush()
	return nil
}

func (s LogStream) trans(r io.Reader, w io.Writer) error {
	decoder := jstream.NewDecoder(r, 0) // extract JSON values at a depth level of 1
	id := 0
	for mv := range decoder.Stream() {
		id++
		vms, err := to_vm(id, mv.Value.(map[string]interface{}))
		if err != nil {
			s.l.Error(err, "Failed to convert to vm")
			continue
		}
		for _, vm := range vms {
			vm_json_compact, err := json.Marshal(vm)
			if err != nil {
				s.l.Error(err, "Failed to marshal vm")
				continue
			}
			w.Write(vm_json_compact)
			w.Write([]byte("\n"))
		}
	}
	return nil
}

func to_vm(id int, m map[string]interface{}) ([]VmAccess, error) {
	// time_local_str:=m
	id_str := strconv.Itoa(id)
	time_msec_str := access_str(m, "msec")
	client_addr_str := access_str(m, "client_addr")
	client_port_str := access_str(m, "client_port")
	server_addr_str := access_str(m, "server_addr")
	server_port_str := access_str(m, "server_port")
	pid_str := access_str(m, "pid")
	rule_id_str := access_str(m, "rule_id")
	host_str := access_str(m, "host")
	request_str := access_str(m, "request")
	status_str := access_str(m, "status")
	upstream_status_str := access_str(m, "upstream_status")
	upstream_addr_str := access_str(m, "upstream_addr")
	user_agent_str := access_str(m, "user_agent")
	x_forward_for_str := access_str(m, "x-forward-for")
	request_time_str := access_str(m, "request_time")
	upstream_response_time_str := access_str(m, "upstream_response_time")
	upstream_bytes_received_str := access_str(m, "upstream_bytes_received")
	time_msec := float_or(time_msec_str, 0)
	time_ms := int64(time_msec * 1000)
	access := VmAccess{
		Metric: Metric{
			Name:           "alb_access",
			Job:            "alb",
			XId:            id_str,
			ClientAddr:     client_addr_str,
			ClientPort:     client_port_str,
			ServerAddr:     server_addr_str,
			ServerPort:     server_port_str,
			PID:            pid_str,
			RuleID:         rule_id_str,
			Host:           host_str,
			Request:        request_str,
			Status:         status_str,
			UpstreamStatus: upstream_status_str,
			UpstreamAddr:   upstream_addr_str,
			UserAgent:      user_agent_str,
			XForwardFor:    x_forward_for_str,
		},
		Values:     []float64{1},
		Timestamps: []int64{time_ms},
	}
	access_request_time := VmAccess{
		Metric: Metric{
			Name:           "alb_access_request_time",
			Job:            "alb",
			XId:            id_str,
			ClientAddr:     client_addr_str,
			ClientPort:     client_port_str,
			ServerAddr:     server_addr_str,
			ServerPort:     server_port_str,
			PID:            pid_str,
			RuleID:         rule_id_str,
			Host:           host_str,
			Request:        request_str,
			Status:         status_str,
			UpstreamStatus: upstream_status_str,
			UpstreamAddr:   upstream_addr_str,
			UserAgent:      user_agent_str,
			XForwardFor:    x_forward_for_str,
		},
		Values:     []float64{float_or(request_time_str, -1)},
		Timestamps: []int64{time_ms},
	}
	access_upstream_response_time := VmAccess{
		Metric: Metric{
			Name:           "alb_access_upstream_response_time",
			Job:            "alb",
			XId:            id_str,
			ClientAddr:     client_addr_str,
			ClientPort:     client_port_str,
			ServerAddr:     server_addr_str,
			ServerPort:     server_port_str,
			PID:            pid_str,
			RuleID:         rule_id_str,
			Host:           host_str,
			Request:        request_str,
			Status:         status_str,
			UpstreamStatus: upstream_status_str,
			UpstreamAddr:   upstream_addr_str,
			UserAgent:      user_agent_str,
			XForwardFor:    x_forward_for_str,
		},
		Values:     []float64{float_or(upstream_response_time_str, -1)},
		Timestamps: []int64{time_ms},
	}
	access_upstream_bytes_received := VmAccess{
		Metric: Metric{
			Name:           "alb_access_upstream_bytes_received",
			Job:            "alb",
			XId:            id_str,
			ClientAddr:     client_addr_str,
			ClientPort:     client_port_str,
			ServerAddr:     server_addr_str,
			ServerPort:     server_port_str,
			PID:            pid_str,
			RuleID:         rule_id_str,
			Host:           host_str,
			Request:        request_str,
			Status:         status_str,
			UpstreamStatus: upstream_status_str,
			UpstreamAddr:   upstream_addr_str,
			UserAgent:      user_agent_str,
			XForwardFor:    x_forward_for_str,
		},
		Values:     []float64{float_or(upstream_bytes_received_str, -1)},
		Timestamps: []int64{time_ms},
	}
	_ = access_request_time
	_ = access_upstream_response_time
	_ = access_upstream_bytes_received
	return []VmAccess{
		access,
		// access_request_time,
		// access_upstream_response_time,
		// access_upstream_bytes_received,
	}, nil
}

func access_str(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		return v.(string)
	}
	return ""
}

func int_or(input string, default_value int) int {
	if input == "" {
		return default_value
	}
	i, err := strconv.Atoi(input)
	if err != nil {
		return default_value
	}
	return i
}

func float_or(input string, default_value float64) float64 {
	if input == "" {
		return default_value
	}
	i, err := strconv.ParseFloat(input, 64)
	if err != nil {
		return default_value
	}
	return i
}
