package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var databaseURL string

var (
	requestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total HTTP requests",
		},
		[]string{"method", "path", "status"},
	)

	requestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request duration in seconds",
		},
		[]string{"method", "path"},
	)
)

type statusRecorder struct {
	http.ResponseWriter
	statusCode int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

func metricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		recorder := &statusRecorder{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start).Seconds()
		path := r.URL.Path
		method := r.Method
		status := strconv.Itoa(recorder.statusCode)

		requestCount.WithLabelValues(method, path, status).Inc()
		requestDuration.WithLabelValues(method, path).Observe(duration)
	})
}

func jsonResponse(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func initDB() {
	if databaseURL == "" {
		return
	}
	conn, err := pgx.Connect(context.Background(), databaseURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v", err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(),
		"CREATE TABLE IF NOT EXISTS visits (id SERIAL PRIMARY KEY, timestamp TIMESTAMPTZ DEFAULT NOW())")
	if err != nil {
		log.Fatalf("Unable to create visits table: %v", err)
	}
	log.Println("Database initialised")
}

func main() {
	databaseURL = os.Getenv("DATABASE_URL")
	initDB()

	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		jsonResponse(w, map[string]string{
			"app":     "sample-app",
			"team":    "team-beta",
			"version": "1.0.0",
		})
	})

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		jsonResponse(w, map[string]string{
			"status": "ok",
		})
	})

	mux.HandleFunc("GET /db-health", func(w http.ResponseWriter, r *http.Request) {
		if databaseURL == "" {
			jsonResponse(w, map[string]string{
				"status": "no database configured",
			})
			return
		}
		conn, err := pgx.Connect(context.Background(), databaseURL)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jsonResponse(w, map[string]string{"status": "error", "error": err.Error()})
			return
		}
		defer conn.Close(context.Background())

		_, err = conn.Exec(context.Background(), "INSERT INTO visits DEFAULT VALUES")
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jsonResponse(w, map[string]string{"status": "error", "error": err.Error()})
			return
		}

		var count int64
		err = conn.QueryRow(context.Background(), "SELECT COUNT(*) FROM visits").Scan(&count)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			jsonResponse(w, map[string]string{"status": "error", "error": err.Error()})
			return
		}

		jsonResponse(w, map[string]any{
			"status": "ok",
			"visits": count,
		})
	})

	mux.Handle("GET /metrics", promhttp.Handler())

	handler := metricsMiddleware(mux)

	log.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", handler); err != nil {
		log.Fatal(err)
	}
}
