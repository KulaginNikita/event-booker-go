# EventBooker

Сервис для мероприятий с бронированием мест и дедлайном оплаты. Если бронь не подтверждена за заданное время, фоновый scheduler автоматически переводит ее в `cancelled`, а место снова становится свободным.

## Архитектура

- `cmd/eventbooker` — точка входа.
- `internal/app` — запуск HTTP-сервера, scheduler и graceful shutdown.
- `internal/di` — сборка зависимостей.
- `internal/api/http` — handlers, DTO, converters, router.
- `internal/service` — бизнес-логика и валидация.
- `internal/repository/postgres` — PostgreSQL repository, транзакции, `FOR UPDATE`.
- `internal/scheduler` — cron-like обработчик просроченных броней.
- `internal/notifier` — уведомления об отменах через logger, Email и Telegram.
- `internal/domain` — чистые доменные структуры без transport-тегов.
- `pkg/closer`, `pkg/logger` — инфраструктурные пакеты.

## HTTP API

- `POST /events` — создать мероприятие.
- `GET /events` — список мероприятий.
- `GET /events/{id}` — детали мероприятия, свободные места и брони.
- `POST /events/{id}/book` — забронировать место.
- `POST /events/{id}/confirm` — подтвердить бронь.

## Web

- `/admin` — создание мероприятий и просмотр статуса.
- `/` — пользовательская страница для бронирования и подтверждения.

## Уведомления

Email и Telegram включаются через `.env` или переменные окружения. Имена переменных совместимы с `notification-service-go`:

```bash
EMAIL_ENABLED=true
EMAIL_SMTP_HOST=smtp.gmail.com
EMAIL_SMTP_PORT=587
EMAIL_USERNAME=your-email@gmail.com
EMAIL_PASSWORD=your-app-password
EMAIL_FROM=your-email@gmail.com

TG_ENABLED=true
TG_TOKEN=123456789:replace-with-token
```

Email отправляется на `user_email` из брони. Telegram отправляется, если при бронировании заполнить поле `user_telegram`; поддерживается chat id или публичный `@channel`.

## Запуск

```bash
docker compose up --build
```

После запуска:

- пользовательская страница: `http://localhost:8080`
- admin-страница: `http://localhost:8080/admin`

## Проверки

```bash
go test ./...
go vet ./...
```
