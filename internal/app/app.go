package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/wb-go/wbf/logger"

	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/di"
	pkglogger "github.com/KulaginNikita/event-booker/pkg/logger"
)

func Run(configPath string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	zapLog, err := pkglogger.New(cfg.Logger, cfg.App.Env)
	if err != nil {
		return fmt.Errorf("init zap logger: %w", err)
	}
	defer zapLog.Sync() //nolint:errcheck

	log, err := logger.InitLogger(logger.ZapEngine, cfg.App.Name, cfg.App.Env, logger.WithLevel(logger.InfoLevel))
	if err != nil {
		return fmt.Errorf("init wbf logger: %w", err)
	}

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
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      container.Router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	container.Closer.Add("http-server", func(ctx context.Context) error {
		return srv.Shutdown(ctx)
	})

	go func() {
		log.Info("starting HTTP server", "port", cfg.HTTP.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP server error", "error", err)
		}
	}()

	return container.Closer.WaitAndClose(10 * time.Second)
}
