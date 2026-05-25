package closer

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

type CloseFunc func(ctx context.Context) error

type closeItem struct {
	name string
	fn   CloseFunc
}

type Closer struct {
	mu     sync.Mutex
	once   sync.Once
	items  []closeItem
	log    *zap.Logger
	result error
}

func New(log *zap.Logger) *Closer {
	if log == nil {
		log = zap.NewNop()
	}
	return &Closer{log: log.Named("closer")}
}

func (c *Closer) Add(name string, fn CloseFunc) {
	if fn == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = append(c.items, closeItem{name: name, fn: fn})
}

func (c *Closer) CloseAll(ctx context.Context) error {
	c.once.Do(func() {
		c.mu.Lock()
		items := make([]closeItem, len(c.items))
		copy(items, c.items)
		c.items = nil
		c.mu.Unlock()

		for i := len(items) - 1; i >= 0; i-- {
			if err := c.closeOne(ctx, items[i]); err != nil {
				c.result = errors.Join(c.result, err)
			}
		}
	})
	return c.result
}

func (c *Closer) WaitAndClose(timeout time.Duration) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	sig := <-stop
	c.log.Info("shutdown signal received", zap.String("signal", sig.String()))

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), timeout)
	defer shutdownCancel()

	if err := c.CloseAll(shutdownCtx); err != nil {
		c.log.Error("shutdown completed with errors", zap.Error(err))
		return err
	}
	c.log.Info("graceful shutdown complete")
	return nil
}

func (c *Closer) closeOne(ctx context.Context, item closeItem) error {
	start := time.Now()
	c.log.Info("closing resource", zap.String("resource", item.name))

	defer func() {
		if rec := recover(); rec != nil {
			c.log.Error("panic while closing resource",
				zap.String("resource", item.name),
				zap.Any("panic", rec),
			)
		}
	}()

	err := item.fn(ctx)
	duration := time.Since(start)
	if err != nil {
		c.log.Error("failed to close resource",
			zap.String("resource", item.name),
			zap.Duration("duration", duration),
			zap.Error(err),
		)
		return err
	}

	c.log.Info("resource closed",
		zap.String("resource", item.name),
		zap.Duration("duration", duration),
	)
	return nil
}
