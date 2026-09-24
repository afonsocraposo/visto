package httpserver

import (
	"encoding/json"
	"net/http"
	"time"
)

type Server struct {
	handler http.Handler
}

func New() *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", health)
	return &Server{handler: mux}
}

func (server *Server) Handler() http.Handler {
	return server.handler
}

func health(response http.ResponseWriter, _ *http.Request) {
	response.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(response).Encode(map[string]string{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}
