package ui

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed web/*
var webAssets embed.FS

type Server struct {
	handler http.Handler
}

func NewServer() (*Server, error) {
	assets, err := fs.Sub(webAssets, "web")
	if err != nil {
		return nil, err
	}
	server := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("/api/runs", server.handleRuns)
	mux.HandleFunc("/api/runs/", server.handleRun)
	mux.HandleFunc("/api/stream", server.handleStream)
	mux.Handle("/", http.FileServer(http.FS(assets)))
	server.handler = securityHeaders(mux)
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.handler
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	snapshots, err := LoadSnapshots()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	summaries := make([]RunSummary, len(snapshots))
	for i := range snapshots {
		summaries[i] = snapshots[i].Summary
	}
	writeJSON(w, summaries)
}

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	if id == "" || strings.Contains(id, "/") {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid run ID"))
		return
	}
	snapshots, err := LoadSnapshots()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	for _, snapshot := range snapshots {
		if snapshot.Summary.ID == id {
			writeJSON(w, snapshot)
			return
		}
	}
	writeError(w, http.StatusNotFound, fmt.Errorf("run not found"))
}

func (s *Server) handleStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastDigest := ""
	send := func() bool {
		snapshots, err := LoadSnapshots()
		if err != nil {
			fmt.Fprintf(w, "event: error\ndata: %q\n\n", err.Error())
			flusher.Flush()
			return true
		}
		raw, _ := json.Marshal(snapshots)
		sum := sha256.Sum256(raw)
		digest := hex.EncodeToString(sum[:])
		if digest == lastDigest {
			return true
		}
		lastDigest = digest
		fmt.Fprintf(w, "event: runs\ndata: %s\n\n", raw)
		flusher.Flush()
		return true
	}
	send()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticker.C:
			if !send() {
				return
			}
		}
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
