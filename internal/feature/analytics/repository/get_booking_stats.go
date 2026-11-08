package analytics_repository

import (
	"context"
	"fmt"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/google/uuid"
)

func (a *Analytics) GetBookingStats(
	ctx context.Context,
	eventID uuid.UUID,
	from, to time.Time,
) ([]BookingStatsRow, error) {
	ctx, cancel := context.WithTimeout(ctx, a.OpTimeout)
	defer cancel()

	const query = `
	SELECT event_type, uniqExact(booking_id) AS bookings, sum(quantity) AS tickets
	FROM (
		SELECT DISTINCT message_id, booking_id, event_type, quantity
		FROM booking_events
		WHERE event_id = @event_id
		  AND occurred_at >= @from
		  AND occurred_at < @to
	)
	GROUP BY event_type
	ORDER BY event_type`

	rows, err := a.Conn.Query(ctx, query,
		clickhouse.Named("event_id", eventID),
		clickhouse.DateNamed("from", microsecondCeiling(from), clickhouse.MicroSeconds),
		clickhouse.DateNamed("to", microsecondCeiling(to), clickhouse.MicroSeconds),
	)
	if err != nil {
		return nil, fmt.Errorf("query booking stats: %w", err)
	}
	defer rows.Close()

	stats := make([]BookingStatsRow, 0, 3)
	for rows.Next() {
		var row BookingStatsRow
		if err := rows.Scan(&row.EventType, &row.Bookings, &row.Tickets); err != nil {
			return nil, fmt.Errorf("scan booking stats: %w", err)
		}
		stats = append(stats, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read booking stats: %w", err)
	}
	return stats, nil
}

// Stored timestamps have microsecond precision. Rounding both bounds up keeps
// the exact [from, to) selection even when the caller supplies nanoseconds.
func microsecondCeiling(value time.Time) time.Time {
	value = value.UTC()
	rounded := value.Truncate(time.Microsecond)
	if rounded.Before(value) {
		return rounded.Add(time.Microsecond)
	}
	return rounded
}
