package events_transport_test

import (
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	"github.com/stretchr/testify/require"
)

func TestCreateEventJSONV2StrictInput(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"duplicate field", `{"name":"First","name":"Second"}`},
		{"case mismatch", `{"Name":"Go Live"}`},
		{"invalid UTF-8", "{\"name\":\"bad\xfftext\"}"},
		{"two values", `{"name":"First"} {"name":"Second"}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stubService{}
			w := httptest.NewRecorder()
			transport.NewEventsHTTPTransport(s).CreateEvent(w, organizerRequest(tc.body))
			require.Equal(t, http.StatusBadRequest, w.Code)
			require.Zero(t, s.calls)
		})
	}
}

func TestUpdateEventJSONV2StrictInput(t *testing.T) {
	for _, body := range []string{
		`{"name":"First","name":"Second"}`,
		`{"name":null,"name":"Second"}`,
		`{"Name":"Go Live"}`,
		`{"description":null}`,
		`{"unexpected":"value"}`,
		"{\"name\":\"bad\xfftext\"}",
		`{"name":"First"} {"name":"Second"}`,
	} {
		var dto transport.UpdateEventRequest
		require.Error(t, json.UnmarshalRead(strings.NewReader(body), &dto, json.RejectUnknownMembers(true)), body)
	}
	var dto transport.UpdateEventRequest
	require.NoError(t, json.Unmarshal([]byte(`{"description":""}`), &dto, json.RejectUnknownMembers(true)))
	require.Nil(t, dto.Name)
	require.NotNil(t, dto.Description)
	require.Empty(t, *dto.Description)
	// A nil list uses [] under v2; keep the API's items array contract explicit.
	data, err := json.Marshal(transport.EventsListResponse{Items: nil, Limit: 20})
	require.NoError(t, err)
	require.JSONEq(t, `{"items":[],"limit":20,"offset":0}`, string(data))
}
