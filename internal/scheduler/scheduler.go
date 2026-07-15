package scheduler

import (
	"context"
	"time"

	"go.uber.org/zap"
)

type Expirer interface {
	CancelExpired(ctx context.Context) (int, error)
}

type Scheduler struct {
	service  Expirer
	interval time.Duration
	log      *zap.SugaredLogger
}

func New(service Expirer, interval time.Duration, log *zap.SugaredLogger) *Scheduler {
	return &Scheduler{service: service, interval: interval, log: log}
}

func (s *Scheduler) Run(ctx context.Context) {
	if s.interval <= 0 {
		s.interval = 10 * time.Second
	}

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	s.tick(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	count, err := s.service.CancelExpired(ctx)
	if err != nil {
		s.log.Errorw("cancel expired bookings", "error", err)
		return
	}
	if count > 0 {
		s.log.Infow("expired bookings cancelled", "count", count)
	}
}
