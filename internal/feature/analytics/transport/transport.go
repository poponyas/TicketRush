package analytics_transport

import (
	"context"

	service "github.com/poponyas/TicketRush/internal/feature/analytics/service"
)

type HTTPAnalyticsTransport struct {
	analyticsService AnalyticsService
}

type AnalyticsService interface {
	GetBookingStats(ctx context.Context, input service.BookingStatsInput) (service.BookingStatsReport, error)
}

func NewHTTPAnalyticsTransport(service AnalyticsService) *HTTPAnalyticsTransport {
	return &HTTPAnalyticsTransport{analyticsService: service}
}
