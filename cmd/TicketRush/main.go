package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/poponyas/TicketRush/internal/core/auth"
	"github.com/poponyas/TicketRush/internal/core/domain"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	core_auth_password "github.com/poponyas/TicketRush/internal/core/security/password"
	auth_token "github.com/poponyas/TicketRush/internal/core/security/token"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	auth_repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
	auth_service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	auth_http_transport "github.com/poponyas/TicketRush/internal/feature/identity/transport"
	ticket_type_repository "github.com/poponyas/TicketRush/internal/feature/rate/repository"
	ticket_type_service "github.com/poponyas/TicketRush/internal/feature/rate/service"
	ticket_type_transport "github.com/poponyas/TicketRush/internal/feature/rate/transport"
	session_repository "github.com/poponyas/TicketRush/internal/feature/session/repository"
	session_service "github.com/poponyas/TicketRush/internal/feature/session/service"
	session_transport "github.com/poponyas/TicketRush/internal/feature/session/transport"
	venue_repository "github.com/poponyas/TicketRush/internal/feature/venue/repository"
	venue_service "github.com/poponyas/TicketRush/internal/feature/venue/service"
	venue_http_tranpsort "github.com/poponyas/TicketRush/internal/feature/venue/tranpsort"
	"go.uber.org/zap"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("load env: %w", err)
	}

	jwtConfig, err := auth_token.LoadJWTConfig()
	if err != nil {
		return fmt.Errorf("initialize JWT: %w", err)
	}

	tokenManager, err := auth_token.NewManager(
		jwtConfig.Secret,
		"ticketrush",
		"ticketrush-api",
		15*time.Minute,
	)
	if err != nil {
		return fmt.Errorf("initialize JWT: %w", err)
	}

	loggerConfig, err := corelogger.NewConfig()
	if err != nil {
		return err
	}
	log, err := corelogger.NewLogger(loggerConfig)
	if err != nil {
		return err
	}
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Fprintln(os.Stderr, "close logger:", err)
		}
	}()
	dbConfig, err := postgres.NewConfig()
	if err != nil {
		return err
	}
	pool, err := postgres.NewPool(ctx, dbConfig)
	if err != nil {
		return err
	}
	defer pool.Pool.Close()
	address := os.Getenv("HTTP_ADDR")
	if address == "" {
		address = "127.0.0.1:8080"
	}
	devEnabled := false
	if value := os.Getenv("DEV_AUTH_ENABLED"); value != "" {
		devEnabled, err = strconv.ParseBool(value)
		if err != nil {
			return fmt.Errorf("parse DEV_AUTH_ENABLED: %w", err)
		}
	}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			requestID := middleware.GetReqID(req.Context())
			w.Header().Set(middleware.RequestIDHeader, requestID)
			requestLog := log.With(zap.String("request_id", requestID))
			next.ServeHTTP(w, req.WithContext(corelogger.ToContext(req.Context(), requestLog)))
		})
	})
	r.Use(middleware.Recoverer)
	if devEnabled {
		host, _, err := net.SplitHostPort(address)
		if err != nil || !net.ParseIP(host).IsLoopback() {
			return fmt.Errorf("dev authentication requires a loopback HTTP_ADDR")
		}
		userID, err := uuid.Parse(os.Getenv("DEV_USER_ID"))
		if err != nil || userID == uuid.Nil {
			return fmt.Errorf("DEV_USER_ID must be a nonzero UUID")
		}
		roles := []domain.Role{domain.RoleBuyer, domain.RoleOrganizer}
		if raw := os.Getenv("DEV_ROLES"); raw != "" {
			roles = nil
			for _, value := range strings.Split(raw, ",") {
				role := domain.Role(strings.TrimSpace(value))
				switch role {
				case domain.RoleBuyer, domain.RoleOrganizer, domain.RoleController, domain.RoleVenueOwner, domain.RoleAdmin:
					roles = append(roles, role)
				default:
					return fmt.Errorf("DEV_ROLES contains an unsupported role %q", value)
				}
			}
		}
		identity := auth.Identity{UserID: userID, Roles: roles}
		if identity.HasRole(domain.RoleOrganizer) {
			organizerID, err := uuid.Parse(os.Getenv("DEV_ORGANIZER_ID"))
			if err != nil || organizerID == uuid.Nil {
				return fmt.Errorf("DEV_ORGANIZER_ID must be a nonzero UUID for the organizer role")
			}
			identity.OrganizerID = organizerID
		}
		r.Use(auth.DevIdentity(identity))
		log.Warn("local dev authentication enabled")
	}

	//  ----------------------------- AUTH -------------------------------------
	hasher, err := core_auth_password.NewHasher(core_auth_password.DefaultConfig())
	if err != nil {
		return fmt.Errorf("create password hasher: %w", err)
	}

	authRepo := auth_repository.NewAuthRepo(pool)
	authService, err := auth_service.NewAuthService(authRepo, hasher, tokenManager)
	if err != nil {
		return fmt.Errorf("create auth service: %w", err)
	}
	authTransport := auth_http_transport.NewHTTPAuthTransport(authService)

	r.Post("/api/v1/auth/register", authTransport.Register)
	r.Post("/api/v1/auth/login", authTransport.Login)
	r.Post("/api/v1/auth/refresh", authTransport.Refresh)
	r.With(auth.Authenticate(tokenManager, authService)).
		Post("/api/v1/auth/logout", authTransport.Logout)

	protected := r.With()
	if !devEnabled {
		protected = r.With(auth.Authenticate(tokenManager, authService))
	}
	organizerProtected := protected.With(auth.RequireOrganizer)
	if !devEnabled {
		organizerProtected = protected.With(auth.SelectOrganizer(authService), auth.RequireOrganizer)
	}

	// --------------------------------------------------------------------------

	//  ----------------------------- EVENTS -------------------------------------
	eventTransport := transport.NewEventsHTTPTransport(service.NewEventsService(repository.NewEventsRepo(pool)))
	// Private
	organizerProtected.Post("/api/v1/organizer/events", eventTransport.CreateEvent)
	organizerProtected.Post("/api/v1/organizer/events/{event_id}/publish", eventTransport.PublishEvent)
	organizerProtected.
		Get("/api/v1/organizer/events/{event_id}", eventTransport.GetEvent)
	organizerProtected.
		Get("/api/v1/organizer/events", eventTransport.GetEvents)
	organizerProtected.
		Patch("/api/v1/organizer/events/{event_id}", eventTransport.UpdateEvent)
	organizerProtected.
		Delete("/api/v1/organizer/events/{event_id}", eventTransport.DeleteEvent)
	// Public
	r.Get("/api/v1/events", eventTransport.GetPublicEvents)
	r.Get("/api/v1/events/{event_id}", eventTransport.GetPublicEvent)
	r.Get("/livez", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		pingCtx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := pool.Pool.Ping(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// --------------------------------------------------------------------------

	// ----------------------------- VENUES -------------------------------------
	venueTransport := venue_http_tranpsort.NewHTTPVenueTransport(venue_service.NewVenueService(venue_repository.NewVenueRepository(pool)))
	protected.With(auth.RequireVenueOwner).Post("/api/v1/venues", venueTransport.CreateVenue)
	r.Get("/api/v1/venues", venueTransport.GetVenues)
	r.Get("/api/v1/venues/{venue_id}", venueTransport.GetVenue)

	// --------------------------------------------------------------------------

	// ----------------------------- Sessions -----------------------------------
	sessionTransport := session_transport.NewHTTPSessionTransport(session_service.NewSessionService(session_repository.NewSessionRepository(pool)))
	organizerProtected.Post("/api/v1/organizer/events/{event_id}/sessions", sessionTransport.CreateSession)
	organizerProtected.Get("/api/v1/organizer/events/{event_id}/sessions", sessionTransport.GetSessions)
	organizerProtected.Get("/api/v1/organizer/events/{event_id}/sessions/{session_id}", sessionTransport.GetSession)
	organizerProtected.Patch("/api/v1/organizer/events/{event_id}/sessions/{session_id}", sessionTransport.UpdateSession)
	organizerProtected.Post("/api/v1/organizer/events/{event_id}/sessions/{session_id}/publish", sessionTransport.PublishSession)
	r.Get("/api/v1/events/{event_id}/sessions", sessionTransport.GetPublicSessions)
	r.Get("/api/v1/events/{event_id}/sessions/{session_id}", sessionTransport.GetPublicSession)

	// --------------------------------------------------------------------------

	//  ----------------------------- Rate -------------------------------------
	rateTransport := ticket_type_transport.NewTicketTypeService(ticket_type_service.NewTicketTypeService(ticket_type_repository.NewTicketTypePool(pool)))
	organizerProtected.Post("/api/v1/organizer/events/{event_id}/sessions/{session_id}/ticket-types", rateTransport.CreateTicketType)
	organizerProtected.Get("/api/v1/organizer/events/{event_id}/sessions/{session_id}/ticket-types/{ticket_type_id}", rateTransport.GetTicketType)
	organizerProtected.Get("/api/v1/organizer/events/{event_id}/sessions/{session_id}/ticket-types", rateTransport.GetTicketTypes)
	r.Get("/api/v1/events/{event_id}/sessions/{session_id}/ticket-types", rateTransport.GetPublicTicketTypes)
	r.Get("/api/v1/events/{event_id}/sessions/{session_id}/ticket-types/{ticket_type_id}", rateTransport.GetPublicTicketType)
	// --------------------------------------------------------------------------

	server := &http.Server{Addr: address, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.ListenAndServe() }()
	log.Info("HTTP server starting", zap.String("address", address))
	select {
	case err := <-serveErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			_ = server.Close()
			return fmt.Errorf("shutdown server: %w", err)
		}
		return nil
	}
}
