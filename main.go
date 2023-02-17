package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"

	"github.com/cmwylie19/prometheus-workshop/utils"
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

// HealthCheckHandler returns a 200 if the server is up
func HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Header().Set("Content-Type", "application/json")

	io.WriteString(w, `{"alive": true}`)
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

func EnableCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Accept-Encoding, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

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
	router.Use(EnableCors)

	// Static files
	fs := http.FileServer(http.Dir("./static"))

	// metrics endpoint
	router.Path("/metrics").Handler(promhttp.Handler())

	// web app
	router.PathPrefix("/web/").Handler(http.StripPrefix("/web/", hitCounterMiddleware(fs)))

	// health check endpoint
	router.Path("/healthz").HandlerFunc(HealthCheckHandler)

	// hits at the web app endpoint
	router.Path("/hits").HandlerFunc(handleHit)

	utils.WriteLog("INFO", fmt.Sprintf("Server started at port %s", utils.GetPort()))
	err := http.ListenAndServe(":"+utils.GetPort(), router)
	if err != nil {
		utils.WriteLog("ERROR", err.Error())
		log.Fatal(err)
	}

}
