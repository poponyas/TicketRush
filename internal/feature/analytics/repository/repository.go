package analytics_repository

import (
	"time"

	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
)

type Analytics struct {
	Conn      driver.Conn
	OpTimeout time.Duration
}

func NewAnalytics(
	conn driver.Conn,
	opTimeout time.Duration,
) *Analytics {
	return &Analytics{
		Conn:      conn,
		OpTimeout: opTimeout,
	}
}
