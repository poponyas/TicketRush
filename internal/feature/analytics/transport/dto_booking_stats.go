package analytics_transport

import (
	"time"

	"github.com/google/uuid"
	service "github.com/poponyas/TicketRush/internal/feature/analytics/service"
)

type BookingStatsResponse struct {
	Bookings uint64 `json:"bookings"`
	Tickets  uint64 `json:"tickets"`
}

type BookingStatsReportResponse struct {
	EventID   uuid.UUID            `json:"event_id"`
	From      time.Time            `json:"from"`
	To        time.Time            `json:"to"`
	Created   BookingStatsResponse `json:"created"`
	Cancelled BookingStatsResponse `json:"cancelled"`
	Expired   BookingStatsResponse `json:"expired"`
}

func bookingStatsResponse(report service.BookingStatsReport) BookingStatsReportResponse {
	return BookingStatsReportResponse{
		EventID: report.EventID,
		From:    report.From,
		To:      report.To,
		Created: BookingStatsResponse{
			Bookings: report.Created.Bookings, Tickets: report.Created.Tickets,
		},
		Cancelled: BookingStatsResponse{
			Bookings: report.Cancelled.Bookings, Tickets: report.Cancelled.Tickets,
		},
		Expired: BookingStatsResponse{
			Bookings: report.Expired.Bookings, Tickets: report.Expired.Tickets,
		},
	}
}
