package analytics

// BookingStatsRow is an aggregate read model shared by the repository and service.
type BookingStatsRow struct {
	EventType string
	Bookings  uint64
	Tickets   uint64
}
