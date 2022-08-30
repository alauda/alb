package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	AmILeader = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "alb_leader",
		Help: "is this pod is leader",
	})
)

func BecomeLeader() {
	AmILeader.Inc()
}

func LoseLeader() {
	AmILeader.Desc()
}

func Handler() http.Handler {
	return promhttp.Handler()
}
