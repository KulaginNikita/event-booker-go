package service

type Metrics interface {
	EventCreated()
	BookingCreated()
	BookingConfirmed()
	BookingsCancelled(count int)
	NotificationSent(eventType string)
	NotificationRescheduled(eventType string)
	NotificationFailed(eventType string)
}

type noopMetrics struct{}

func (noopMetrics) EventCreated()                    {}
func (noopMetrics) BookingCreated()                  {}
func (noopMetrics) BookingConfirmed()                {}
func (noopMetrics) BookingsCancelled(_ int)          {}
func (noopMetrics) NotificationSent(_ string)        {}
func (noopMetrics) NotificationRescheduled(_ string) {}
func (noopMetrics) NotificationFailed(_ string)      {}
