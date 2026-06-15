package httpserver

import (
	"context"
	"net/http"
	"time"

	"ledger-api/app/internal/handlers"
)

// NewServer creates an HTTP server bound to addr using routes from the handler registry.
func NewServer(addr, jwksURL string, allowedOrigins []string, hdlrs *handlers.Registry) *Server {
	router := NewRouter(hdlrs, jwksURL, allowedOrigins)

	return &Server{
		httpServer: &http.Server{
			Addr:         addr,
			Handler:      router.engine,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Stop(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
