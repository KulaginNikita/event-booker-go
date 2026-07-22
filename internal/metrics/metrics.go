package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics struct {
	registry *prometheus.Registry

	httpRequestsTotal   *prometheus.CounterVec
	httpRequestDuration *prometheus.HistogramVec

	eventsTotal        prometheus.Counter
	bookingsTotal      *prometheus.CounterVec
	notificationsTotal *prometheus.CounterVec
}

func New() *Metrics {
	m := &Metrics{
		registry: prometheus.NewRegistry(),
		httpRequestsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "event_booker_http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
		httpRequestDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "event_booker_http_request_duration_seconds",
			Help:    "HTTP request duration in seconds.",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path", "status"}),
		eventsTotal: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "event_booker_events_created_total",
			Help: "Total number of created events.",
		}),
		bookingsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "event_booker_bookings_total",
			Help: "Total number of booking state changes.",
		}, []string{"action"}),
		notificationsTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "event_booker_notifications_total",
			Help: "Total number of notification outbox dispatch results.",
		}, []string{"type", "status"}),
	}

	m.registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
		m.httpRequestsTotal,
		m.httpRequestDuration,
		m.eventsTotal,
		m.bookingsTotal,
		m.notificationsTotal,
	)

	return m
}

func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{})
}

func (m *Metrics) ObserveHTTPRequest(method string, path string, status int, duration time.Duration) {
	labels := prometheus.Labels{
		"method": method,
		"path":   path,
		"status": strconv.Itoa(status),
	}
	m.httpRequestsTotal.With(labels).Inc()
	m.httpRequestDuration.With(labels).Observe(duration.Seconds())
}

func (m *Metrics) EventCreated() {
	m.eventsTotal.Inc()
}

func (m *Metrics) BookingCreated() {
	m.bookingsTotal.WithLabelValues("created").Inc()
}

func (m *Metrics) BookingConfirmed() {
	m.bookingsTotal.WithLabelValues("confirmed").Inc()
}

func (m *Metrics) BookingsCancelled(count int) {
	if count > 0 {
		m.bookingsTotal.WithLabelValues("cancelled").Add(float64(count))
	}
}

func (m *Metrics) NotificationSent(eventType string) {
	m.notificationsTotal.WithLabelValues(eventType, "sent").Inc()
}

func (m *Metrics) NotificationRescheduled(eventType string) {
	m.notificationsTotal.WithLabelValues(eventType, "rescheduled").Inc()
}

func (m *Metrics) NotificationFailed(eventType string) {
	m.notificationsTotal.WithLabelValues(eventType, "failed").Inc()
}
