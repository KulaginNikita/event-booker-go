package di

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"

	_ "github.com/jackc/pgx/v5/stdlib"
	"go.uber.org/zap"

	httpapi "github.com/KulaginNikita/event-booker/internal/api/http"
	"github.com/KulaginNikita/event-booker/internal/config"
	"github.com/KulaginNikita/event-booker/internal/metrics"
	"github.com/KulaginNikita/event-booker/internal/migrator"
	"github.com/KulaginNikita/event-booker/internal/notifier"
	pgrepo "github.com/KulaginNikita/event-booker/internal/repository/postgres"
	"github.com/KulaginNikita/event-booker/internal/scheduler"
	"github.com/KulaginNikita/event-booker/internal/service"
	"github.com/KulaginNikita/event-booker/pkg/closer"
)

type Container struct {
	ctx       context.Context
	Config    *config.Config
	Logger    *zap.SugaredLogger
	ZapLogger *zap.Logger
	Closer    *closer.Closer
	Repo      *pgrepo.EventRepository
	Metrics   *metrics.Metrics
	Notifier  *notifier.MultiNotifier
	Service   *service.EventService
	Auth      *service.AuthService
	Health    *service.HealthService
	Scheduler *scheduler.Scheduler
	Handler   *httpapi.Handler
	Router    http.Handler
}

func NewContainer(parentCtx context.Context, cfg *config.Config, log *zap.SugaredLogger, zapLog *zap.Logger) (*Container, error) {
	ctx, cancelTelegram := context.WithCancel(parentCtx)
	c := &Container{
		ctx:       ctx,
		Config:    cfg,
		Logger:    log,
		ZapLogger: zapLog,
		Closer:    closer.New(zapLog),
	}
	c.Closer.Add("telegram-listener", func(_ context.Context) error {
		cancelTelegram()
		return nil
	})

	if err := c.initPostgres(); err != nil {
		return nil, fmt.Errorf("init postgres: %w", err)
	}
	if err := c.initRepositories(); err != nil {
		return nil, fmt.Errorf("init repositories: %w", err)
	}
	c.initMetrics()
	c.initServices()
	c.initScheduler()
	c.initHTTP()

	return c, nil
}

func (c *Container) initPostgres() error {
	if err := c.runMigrations(); err != nil {
		return err
	}

	repo, err := pgrepo.NewEventRepository(context.Background(), c.Config.Postgres.DSN, c.Config.Postgres.MaxPoolSize)
	if err != nil {
		return err
	}

	c.Repo = repo
	c.Closer.Add("postgres", func(_ context.Context) error {
		repo.Close()
		return nil
	})
	return nil
}

func (c *Container) runMigrations() error {
	db, err := sql.Open("pgx", c.Config.Postgres.DSN)
	if err != nil {
		return fmt.Errorf("open migration db: %w", err)
	}
	defer db.Close()

	if err := migrator.NewMigrator(db, c.Config.Migration.Dir).Up(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return nil
}

func (c *Container) initRepositories() error {
	return nil
}

func (c *Container) initMetrics() {
	c.Metrics = metrics.New()
}

func (c *Container) initServices() {
	emailNotifier := notifier.NewEmail(c.Config.Email)
	telegramNotifier, err := notifier.NewTelegram(c.ctx, c.Config.Telegram, c.Repo, c.Logger)
	if err != nil {
		c.Logger.Errorw("telegram notifier disabled", "error", err)
	}
	c.Notifier = notifier.NewMulti(c.Logger,
		notifier.NewChannel("log", notifier.NewLogNotifier(c.Logger)),
		notifier.NewChannel("email", emailNotifier),
		notifier.NewChannel("telegram", telegramNotifier),
	)
	c.Service = service.NewEventService(c.Repo, c.Notifier, c.Config.Booking.PaymentDeadline, c.Logger, c.Metrics)
	c.Auth = service.NewAuthService(c.Config.Auth.JWTSecret, c.Config.Auth.TokenTTL, c.Config.Auth.Issuer, c.Config.Auth.Audience, c.Repo)
	c.Health = service.NewHealthService(c.Repo)
}

func (c *Container) initScheduler() {
	c.Scheduler = scheduler.New(c.Service, c.Config.Scheduler.Interval, c.Logger)
}

func (c *Container) initHTTP() {
	c.Handler = httpapi.NewHandler(c.Service, c.Auth, c.Health, c.Logger)
	c.Router = httpapi.NewRouter(c.Handler, c.Metrics, c.Metrics.Handler())
}
