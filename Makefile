.PHONY: run test vet tidy compose-up compose-down

run:
	go run ./cmd/eventbooker -config config/config.yml

test:
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

compose-up:
	docker compose up --build

compose-down:
	docker compose down
