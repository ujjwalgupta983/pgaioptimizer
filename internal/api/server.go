package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ujjwalgupta983/pgaioptimizer/internal/models"
)

// Server represents the API server for pgaioptimizer.
type Server struct {
	addr   string
	mux    *http.ServeMux
	latest *models.HealthReport // In-memory store for the latest report (for now)
}

// NewServer creates a new API server.
func NewServer(addr string) *Server {
	s := &Server{
		addr: addr,
		mux:  http.NewServeMux(),
	}
	s.routes()
	return s
}

// UpdateLatestReport updates the in-memory latest report.
func (s *Server) UpdateLatestReport(report *models.HealthReport) {
	s.latest = report
}

func (s *Server) routes() {
	s.mux.HandleFunc("/api/health", s.handleHealthCheck)
	s.mux.HandleFunc("/api/report/latest", s.handleLatestReport)

	// Optional: Serve frontend static files
	// s.mux.Handle("/", http.FileServer(http.Dir("./web/dist")))
}

func (s *Server) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

func (s *Server) handleLatestReport(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.latest == nil {
		http.Error(w, `{"error": "no report available"}`, http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(s.latest); err != nil {
		http.Error(w, `{"error": "failed to encode report"}`, http.StatusInternalServerError)
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	fmt.Printf("API Server listening on %s\n", s.addr)
	return http.ListenAndServe(s.addr, s.mux)
}
