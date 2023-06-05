package qps

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	. "alauda.io/alb2/utils/log"
	. "alauda.io/alb2/utils/test_utils"
)

func TestQps(t *testing.T) {
	l := L()
	l.Info("ok")
	ctx, cancel := CtxWithSignalAndTimeout(5 * 60)
	go func() {
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			io.WriteString(w, "hello world")
		})
		http.ListenAndServe(":8090", nil)
	}()
	defer cancel()
	tctx, _ := context.WithTimeout(ctx, time.Second*30)
	r := NewReqProvider("http://localhost:8090", l, tctx)
	r.WithSampleInterval(3)
	r.Start()
	fmt.Printf("test ok")
	s := NewSummaryAssert(r.Summary())
	s.NoError()
	s.QpsAbove(300)

}
