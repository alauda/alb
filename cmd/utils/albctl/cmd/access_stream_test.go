package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bcicen/jstream"
)

func TestToVm(t *testing.T) {
	// f, _ := os.Open("input.json")
	raw := `
{"time_local": "20/Jun/2025:15:50:22 +0800","client_addr": "127.0.0.1","client_port": "49070","server_addr": "127.0.0.1","server_port": "80","pid": "216538","rule_id": "1","host": "127.0.0.1","request": "GET /t1 HTTP/1.1","status": "200","upstream_status": "200","upstream_addr": "172.18.0.2:8001","user_agent": "Grafana k6/1.0.0","x-forward-for": "","request_time": "0.050","upstream_response_time": "0.050","upstream_bytes_received": "175"}
{"time_local": "20/Jun/2025:15:50:22 +0800","client_addr": "127.0.0.1","client_port": "49062","server_addr": "127.0.0.1","server_port": "80","pid": "216538","rule_id": "1","host": "127.0.0.1","request": "GET /t1 HTTP/1.1","status": "200","upstream_status": "200","upstream_addr": "172.18.0.2:8001","user_agent": "Grafana k6/1.0.0","x-forward-for": "","request_time": "0.050","upstream_response_time": "0.050","upstream_bytes_received": "175"}
{"time_local": "20/Jun/2025:15:50:22 +0800","client_addr": "127.0.0.1","client_port": "49050","server_addr": "127.0.0.1","server_port": "80","pid": "216538","rule_id": "1","host": "127.0.0.1","request": "GET /t1 HTTP/1.1","status": "200","upstream_status": "200","upstream_addr": "172.18.0.2:8001","user_agent": "Grafana k6/1.0.0","x-forward-for": "","request_time": "0.050","upstream_response_time": "0.050","upstream_bytes_received": "175"}
	`
	r := strings.NewReader(raw)
	wf := bytes.NewBuffer(nil)
	w := bufio.NewWriter(wf)
	decoder := jstream.NewDecoder(r, 0) // extract JSON values at a depth level of 1
	id := 0
	for mv := range decoder.Stream() {
		id++
		vms, err := to_vm(id, mv.Value.(map[string]interface{}))
		if err != nil {
			t.Errorf("Failed to convert to vm: %v", err)
			continue
		}
		for _, vm := range vms {
			vm_json_compact, err := json.Marshal(vm)
			if err != nil {
				t.Errorf("Failed to marshal vm: %v", err)
				continue
			}
			w.Write(vm_json_compact)
			w.WriteByte('\n')
		}
	}
	w.Flush()
	fmt.Printf("%s\n", wf.String())
}
