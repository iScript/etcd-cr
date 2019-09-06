package v3rpc

import "github.com/prometheus/client_golang/prometheus"

var (
	sentBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "network",
		Name:      "client_grpc_sent_bytes_total",
		Help:      "The total number of bytes sent to grpc clients.",
	})

	receivedBytes = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd",
		Subsystem: "network",
		Name:      "client_grpc_received_bytes_total",
		Help:      "The total number of bytes received from grpc clients.",
	})
)
