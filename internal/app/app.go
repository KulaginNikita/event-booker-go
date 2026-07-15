package app

import (
	"context"
	"fmt"
	"net/http"

	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/di"
	pkglogger "github.com/KulaginNikita/event-booker/pkg/logger"
)

func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	zapLog, err := pkglogger.New(cfg.Logger, cfg.App.Env)
	if err != nil {
		return fmt.Errorf("init zap logger: %w", err)
	}
	defer zapLog.Sync() //nolint:errcheck
	log := zapLog.Sugar().Named(cfg.App.Name)

	container, err := di.NewContainer(context.Background(), cfg, log, zapLog)
	if err != nil {
		return fmt.Errorf("init di container: %w", err)
	}

	schedulerCtx, stopScheduler := context.WithCancel(context.Background())
	container.Closer.Add("scheduler", func(_ context.Context) error {
		stopScheduler()
		return nil
	})
	go container.Scheduler.Run(schedulerCtx)

	srv := &http.Server{
		Addr:              ":" + cfg.HTTP.Port,
		Handler:           container.Router,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}
	container.Closer.Add("http-server", func(ctx context.Context) error {
		shutdownCtx, cancel := context.WithTimeout(ctx, cfg.HTTP.ShutdownTimeout)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	})

	go func() {
		log.Infow("starting HTTP server", "port", cfg.HTTP.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Errorw("HTTP server error", "error", err)
		}
	}()

	return container.Closer.WaitAndClose(cfg.HTTP.ShutdownTimeout)
}
