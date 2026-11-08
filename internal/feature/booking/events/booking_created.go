package booking_events

const (
	BookingCreatedType          = "booking.created"
	BookingCreatedSchemaVersion = 1
)

type BookingCreatedPayload = BookingLifecyclePayload
