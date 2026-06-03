package apiserver

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// dispatchTotal counts webhook-triggered Nomad dispatch attempts by outcome
// (dispatched, skipped, error). Exposed on /metrics via the default registry.
var dispatchTotal = promauto.NewCounterVec(
	prometheus.CounterOpts{
		Name: "amihan_webhook_dispatch_total",
		Help: "Total number of webhook-triggered Nomad job dispatch attempts by result.",
	},
	[]string{"result"},
)
