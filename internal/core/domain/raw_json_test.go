package domain_test

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"testing"

	"github.com/poponyas/TicketRush/internal/core/domain"
	"github.com/stretchr/testify/require"
)

func TestRawJSONValuesRemainObjects(t *testing.T) {
	raw := jsontext.Value(`{"ticket_id":"ticket-1"}`)
	outbox := domain.OutboxEvent{Payload: raw}
	data, err := json.Marshal(outbox)
	require.NoError(t, err)
	var fields map[string]jsontext.Value
	require.NoError(t, json.Unmarshal(data, &fields))
	require.JSONEq(t, string(raw), string(fields["Payload"]))
	var decoded domain.OutboxEvent
	require.NoError(t, json.Unmarshal(data, &decoded))
	require.JSONEq(t, string(raw), string(decoded.Payload))
	data, err = json.Marshal(domain.IdempotencyRecord{Response: raw})
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, &fields))
	require.JSONEq(t, string(raw), string(fields["Response"]))
}
