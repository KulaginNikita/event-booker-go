FROM golang:1.25.7-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o /bin/event-booker ./cmd/eventbooker

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /bin/event-booker /bin/event-booker
COPY web ./web
COPY migrations ./migrations

EXPOSE 8080
CMD ["/bin/event-booker"]
