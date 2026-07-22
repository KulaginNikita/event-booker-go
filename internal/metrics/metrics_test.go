package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestMetricsHandlerExposesCustomCollectors(t *testing.T) {
	m := New()

	m.EventCreated()
	m.BookingCreated()
	m.NotificationSent("booking_created")
	m.ObserveHTTPRequest(http.MethodPost, "/events", http.StatusCreated, 15*time.Millisecond)

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()

	m.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	body := rec.Body.String()
	for _, metric := range []string{
		"event_booker_events_created_total",
		"event_booker_bookings_total",
		"event_booker_notifications_total",
		"event_booker_http_requests_total",
	} {
		if !strings.Contains(body, metric) {
			t.Fatalf("metrics response does not contain %q", metric)
		}
	}
}
