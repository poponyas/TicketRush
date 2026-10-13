package venue_http_tranpsort_test

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	service "github.com/poponyas/TicketRush/internal/feature/venue/service"
	transport "github.com/poponyas/TicketRush/internal/feature/venue/tranpsort"
	"github.com/stretchr/testify/require"
)

type stubVenueService struct{ calls int }

func (s *stubVenueService) GetVenue(context.Context, uuid.UUID) (domain.Venue, error) {
	panic("unexpected GetVenue")
}
func (s *stubVenueService) GetVenues(context.Context, *int, *int) (service.VenuesPage, error) {
	panic("unexpected GetVenues")
}

func (s *stubVenueService) CreateVenue(ctx context.Context, name, city, address string) (domain.Venue, error) {
	s.calls++
	return domain.NewVenue(name, city, address), nil
}

func TestCreateVenueRequiresOwnerRole(t *testing.T) {
	for _, tc := range []struct {
		name     string
		identity *auth.Identity
		status   int
	}{
		{"anonymous", nil, http.StatusUnauthorized},
		{"organizer is not venue owner", &auth.Identity{UserID: uuid.New(), OrganizerID: uuid.New(), Roles: []domain.Role{domain.RoleOrganizer}}, http.StatusForbidden},
		{"owner without organizer", &auth.Identity{UserID: uuid.New(), Roles: []domain.Role{domain.RoleVenueOwner}}, http.StatusCreated},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := &stubVenueService{}
			h := transport.NewHTTPVenueTransport(s)
			router := chi.NewRouter()
			router.With(auth.RequireVenueOwner).Post("/api/v1/venues", h.CreateVenue)
			req := httptest.NewRequest(http.MethodPost, "/api/v1/venues", strings.NewReader(`{"name":"Hall","city":"Moscow","address":"Address"}`))
			req.Header.Set("Content-Type", "application/json")
			if tc.identity != nil {
				req = req.WithContext(auth.WithIdentity(req.Context(), *tc.identity))
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			require.Equal(t, tc.status, w.Code, w.Body.String())
			if tc.status == http.StatusCreated {
				require.Equal(t, 1, s.calls)
				var dto transport.VenueResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &dto))
				require.NotEqual(t, uuid.Nil, dto.ID)
				require.Equal(t, "Hall", dto.Name)
			} else {
				require.Zero(t, s.calls)
			}
		})
	}
}
