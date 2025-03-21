package metrics

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog/log"
	"github.com/yugonline/grind_review_bot/config"
)

// Server represents the metrics server
type Server struct {
	httpServer *http.Server
	config     config.MetricsConfig
}

// New creates a new metrics server
func New(cfg config.MetricsConfig) *Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	return &Server{
		httpServer: &http.Server{
			Addr:    cfg.Address,
			Handler: mux,
		},
		config: cfg,
	}
}

// Start starts the metrics server
func (s *Server) Start() error {
	log.Info().Str("address", s.config.Address).Msg("Starting metrics server")
	if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("metrics server failed: %w", err)
	}
	return nil
}

// Stop stops the metrics server gracefully
func (s *Server) Stop(ctx context.Context) error {
	log.Info().Msg("Stopping metrics server")
	shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("metrics server shutdown failed: %w", err)
	}
	return nil
}
