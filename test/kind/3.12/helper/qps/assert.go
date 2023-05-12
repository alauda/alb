package qps

import (
	"fmt"
)

type QpsSummaryAssert struct {
	summary []History
}

func NewSummaryAssert(sum []History) *QpsSummaryAssert {
	return &QpsSummaryAssert{summary: sum}
}

func (s *QpsSummaryAssert) NoError() {
	for _, s := range s.summary {
		for _, r := range s.records {
			if r.code != 200 {
				panic(fmt.Errorf("find a err %v", r))
			}
		}
	}
}

func (s *QpsSummaryAssert) QpsAbove(qps int) {
	for _, s := range s.summary {
		// qps := s.success / uint64(s.dur)
		cur := s.success / uint64(s.dur)
		if cur < uint64(qps) {
			panic(fmt.Errorf("qps expect >%d but is %d ", qps, cur))
		}
	}
}
