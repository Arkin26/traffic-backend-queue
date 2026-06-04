package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	HTTPRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "traffic_http_requests_total",
		Help: "Total HTTP requests",
	}, []string{"method", "path", "status"})

	QueueJoins = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "traffic_queue_joins_total",
		Help: "Queue join attempts",
	}, []string{"sale_id", "result"})

	QueueDepth = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "traffic_queue_depth",
		Help: "Current queue depth per sale",
	}, []string{"sale_id"})

	Admissions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "traffic_admissions_total",
		Help: "Users admitted from waiting room",
	}, []string{"sale_id"})

	Checkouts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "traffic_checkouts_total",
		Help: "Checkout attempts",
	}, []string{"sale_id", "result"})
)

func Handler() http.Handler {
	return promhttp.Handler()
}
