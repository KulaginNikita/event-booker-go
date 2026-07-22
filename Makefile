.PHONY: run test test-integration vet tidy compose-up compose-down

run:
	go run ./cmd/eventbooker

test:
	go test ./...

test-integration:
	go test -tags=integration ./internal/integration

vet:
	go vet ./...

tidy:
	go mod tidy

compose-up:
	docker compose up --build

compose-down:
	docker compose down
