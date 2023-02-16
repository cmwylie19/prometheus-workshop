package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Total requests per path
var totalRequests = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name:        "http_requests_total",
		Help:        "Number of get requests.",
		ConstLabels: prometheus.Labels{"metrics": "custom"},
	},
	[]string{"path"},
)

// Response statuses
var responseStatus = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name:        "response_status",
		Help:        "Status of HTTP response",
		ConstLabels: prometheus.Labels{"metrics": "custom"},
	},
	[]string{"status"},
)

// Response time per path
var httpDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:        "http_response_time_seconds",
	Help:        "Duration of HTTP requests.",
	ConstLabels: prometheus.Labels{"metrics": "custom"},
}, []string{"path"})

// initial count
var count int = 0

// handleHit returns the number of hits to the web app
func handleHit(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(strconv.Itoa(count)))
}

// Middleware for counting hits to the web app
// This only works if there is one replicas of the backend.
// This data is ephemeral and will be lost if the backend is restarted.
// use a cache like redis to persist the data
func hitCounterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count += 1
		next.ServeHTTP(w, r)
	})
}

// Middleware for prometheus metrics for each endpoint
func prometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		route := mux.CurrentRoute(r)
		path, _ := route.GetPathTemplate()

		timer := prometheus.NewTimer(httpDuration.WithLabelValues(path))
		rw := NewResponseWriter(w)
		next.ServeHTTP(rw, r)

		statusCode := rw.statusCode

		responseStatus.WithLabelValues(strconv.Itoa(statusCode)).Inc()
		totalRequests.WithLabelValues(path).Inc()

		timer.ObserveDuration()
	})
}
func init() {
	// register custom prometheus metrics
	prometheus.Register(totalRequests)
	prometheus.Register(responseStatus)
	prometheus.Register(httpDuration)
}

func main() {

	router := mux.NewRouter()
	router.Use(prometheusMiddleware)

	// Static files
	fs := http.FileServer(http.Dir("./static"))

	// metrics endpoint
	router.Path("/metrics").Handler(promhttp.Handler())

	// web app
	router.Path("/").Handler(hitCounterMiddleware(fs))

	// hits at the web app endpoint
	router.Path("/hits").HandlerFunc(handleHit)

	err := http.ListenAndServe(":2112", router)
	log.Fatal(err)
}