package qps

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
)

type History struct {
	time    string
	dur     int
	count   map[string]uint64
	success uint64
	records []Record
}

type ReqProvider struct {
	log            logr.Logger
	url            string
	sampleinterval int
	ctx            context.Context
	history        []History
	curCount       map[string]uint64 // current
	curRecords     []Record          // current records
}

type Record struct {
	Id      int
	time    int
	code    int
	latency int // ms
}

// id time code latency
// TODO 应该封装下k6之类的性能测试工具。但这个接口可以保持不变。目前能提供2k的qps。测qps变化基本够用了.
func NewReqProvider(url string, log logr.Logger, ctx context.Context) *ReqProvider {
	return &ReqProvider{url: url, log: log, ctx: ctx, curCount: map[string]uint64{}, history: make([]History, 0)}
}

func (r *ReqProvider) WithSampleInterval(samp int) *ReqProvider {
	r.sampleinterval = samp
	return r
}

func errCount(c map[string]uint64) uint64 {
	var sum uint64 = 0
	for k, v := range c {
		if k == "200 OK" {
			continue
		}
		sum += v
	}
	return sum
}

func (r *ReqProvider) sample() (map[string]uint64, []Record) {
	// TODO add lock?
	m := map[string]uint64{}
	for k, v := range r.curCount {
		m[k] = v
	}
	rs := r.curRecords
	r.curCount = map[string]uint64{}
	r.curRecords = []Record{}
	return m, rs
}

func (r *ReqProvider) job() {
	c := http.Client{}
	count := 0
	for {
		count++
		select {
		case <-r.ctx.Done():
			return
		default:
		}
		url := fmt.Sprintf("%s/%d", r.url, count)
		req, err := http.NewRequest("GET", url, nil)
		start := time.Now()
		if err != nil {
			os.Exit(1)
		}
		resp, err := c.Do(req)
		if err != nil {
			r.log.Error(err, "curl url failed")
			r.curCount["err"]++
			continue
		}
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		end := time.Now()
		r.curCount[resp.Status]++
		latency := int(end.Sub(start).Microseconds())

		r.curRecords = append(r.curRecords, Record{Id: count, time: int(time.Now().Unix()), code: resp.StatusCode, latency: latency})
		r.log.V(3).Info("+1", "latency", latency, "code", resp.Status, "count", r.curCount["200 OK"])
		// time.Sleep(time.Duration(1) * time.Millisecond) // max 1k req per second
	}
}

func (r *ReqProvider) Start() {
	r.log.Info("start")
	go r.job()
	sec := 5
	for {
		select {
		case <-r.ctx.Done():
			return
		case <-time.After(time.Duration(sec) * time.Second):
		}
		count, records := r.sample()
		r.history = append(r.history, History{records: records, dur: sec, time: time.Now().Format("2006-01-02 15:04:05"), count: count, success: count["200 OK"]})
		if len(r.history) == 1 {
			cur := r.history[len(r.history)-1]
			qps := float64(cur.success) / float64(sec)
			r.log.Info("status", "qps", qps, "err", errCount(cur.count)/uint64(sec), "count", cur.count)
		} else {
			pre := r.history[len(r.history)-2]
			cur := r.history[len(r.history)-1]
			qps := float64(cur.success) / float64(sec)
			ec := (errCount(cur.count) - errCount(pre.count)) / uint64(sec)
			r.log.Info("status", "qps", qps, "err", ec, "count", cur.count)
		}
	}
}

func (r *ReqProvider) Summary() []History {
	return r.history
}
