package di

import (
	"context"
	"fmt"

	pgxdriver "github.com/wb-go/wbf/dbpg/pgx-driver"
	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/logger"
	"go.uber.org/zap"

	httpapi "github.com/KulaginNikita/event-booker/internal/api/http"
	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/notifier"
	pgrepo "github.com/KulaginNikita/event-booker/internal/repository/postgres"
	"github.com/KulaginNikita/event-booker/internal/scheduler"
	"github.com/KulaginNikita/event-booker/internal/service"
	"github.com/KulaginNikita/event-booker/pkg/closer"
)

type Container struct {
	Config    *config.Config
	Logger    logger.Logger
	ZapLogger *zap.Logger
	Closer    *closer.Closer
	Postgres  *pgxdriver.Postgres
	Repo      *pgrepo.EventRepository
	Notifier  *notifier.MultiNotifier
	Service   *service.EventService
	Scheduler *scheduler.Scheduler
	Handler   *httpapi.Handler
	Router    *ginext.Engine
}

func NewContainer(_ context.Context, cfg *config.Config, log logger.Logger, zapLog *zap.Logger) (*Container, error) {
	c := &Container{
		Config:    cfg,
		Logger:    log,
		ZapLogger: zapLog,
		Closer:    closer.New(zapLog),
	}

	if err := c.initPostgres(); err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}
	if err := c.initRepositories(); err != nil {
		return nil, fmt.Errorf("init repositories: %w", err)
	}
	c.initServices()
	c.initScheduler()
	c.initHTTP()

	return c, nil
}

func (c *Container) initPostgres() error {
	pg, err := pgxdriver.New(
		c.Config.Postgres.DSN,
		c.Logger,
		pgxdriver.MaxPoolSize(c.Config.Postgres.MaxPoolSize),
		pgxdriver.MaxConnAttempts(5),
	)
	if err != nil {
		return err
	}

	c.Postgres = pg
	c.Closer.Add("postgres", func(_ context.Context) error {
		pg.Close()
		return nil
	})
	return nil
}

func (c *Container) initRepositories() error {
	repo, err := pgrepo.NewEventRepository(c.Postgres, c.Logger)
	if err != nil {
		return err
	}
	c.Repo = repo
	return nil
}

func (c *Container) initServices() {
	emailNotifier := notifier.NewEmail(c.Config.Email)
	telegramNotifier, err := notifier.NewTelegram(c.Config.Telegram)
	if err != nil {
		c.Logger.Error("telegram notifier disabled", "error", err)
	}
	c.Notifier = notifier.NewMulti(c.Logger, notifier.NewLogNotifier(c.Logger), emailNotifier, telegramNotifier)
	c.Service = service.NewEventService(c.Repo, c.Notifier, c.Config.Booking.PaymentDeadline, c.Logger)
}

func (c *Container) initScheduler() {
	c.Scheduler = scheduler.New(c.Service, c.Config.Scheduler.Interval, c.Logger)
}

func (c *Container) initHTTP() {
	c.Handler = httpapi.NewHandler(c.Service, c.Logger)
	c.Router = httpapi.NewRouter(c.Handler)
}
