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
	core_cache_redis "github.com/poponyas/TicketRush/internal/core/cache/redis"
	"github.com/poponyas/TicketRush/internal/core/domain"
	corelogger "github.com/poponyas/TicketRush/internal/core/logger"
	core_metrics "github.com/poponyas/TicketRush/internal/core/metrics"
	core_ratelimit "github.com/poponyas/TicketRush/internal/core/ratelimit"
	core_clickhouse "github.com/poponyas/TicketRush/internal/core/repository/clickhouse"
	postgres "github.com/poponyas/TicketRush/internal/core/repository/postgres"
	core_auth_password "github.com/poponyas/TicketRush/internal/core/security/password"
	auth_token "github.com/poponyas/TicketRush/internal/core/security/token"
	core_tracing "github.com/poponyas/TicketRush/internal/core/tracing"
	analytics_repository "github.com/poponyas/TicketRush/internal/feature/analytics/repository"
	analytics_service "github.com/poponyas/TicketRush/internal/feature/analytics/service"
	analytics_transport "github.com/poponyas/TicketRush/internal/feature/analytics/transport"
	booking_repository "github.com/poponyas/TicketRush/internal/feature/booking/repository"
	booking_service "github.com/poponyas/TicketRush/internal/feature/booking/service"
	booking_transport "github.com/poponyas/TicketRush/internal/feature/booking/transport"
	booking_worker "github.com/poponyas/TicketRush/internal/feature/booking/worker"
	events_cache "github.com/poponyas/TicketRush/internal/feature/events/cache"
	repository "github.com/poponyas/TicketRush/internal/feature/events/repository"
	service "github.com/poponyas/TicketRush/internal/feature/events/service"
	transport "github.com/poponyas/TicketRush/internal/feature/events/transport"
	auth_repository "github.com/poponyas/TicketRush/internal/feature/identity/repository"
	auth_service "github.com/poponyas/TicketRush/internal/feature/identity/service"
	auth_http_transport "github.com/poponyas/TicketRush/internal/feature/identity/transport"
	outbox_publisher "github.com/poponyas/TicketRush/internal/feature/outbox/publisher"
	outbox_repository "github.com/poponyas/TicketRush/internal/feature/outbox/repository"
	outbox_worker "github.com/poponyas/TicketRush/internal/feature/outbox/worker"
	ticket_type_repository "github.com/poponyas/TicketRush/internal/feature/rate/repository"
	ticket_type_service "github.com/poponyas/TicketRush/internal/feature/rate/service"
	ticket_type_transport "github.com/poponyas/TicketRush/internal/feature/rate/transport"
	session_repository "github.com/poponyas/TicketRush/internal/feature/session/repository"
	session_service "github.com/poponyas/TicketRush/internal/feature/session/service"
	session_transport "github.com/poponyas/TicketRush/internal/feature/session/transport"
	venue_repository "github.com/poponyas/TicketRush/internal/feature/venue/repository"
	venue_service "github.com/poponyas/TicketRush/internal/feature/venue/service"
	venue_http_tranpsort "github.com/poponyas/TicketRush/internal/feature/venue/tranpsort"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
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

	tracerProvider, err := core_tracing.NewTracerProvider(ctx)
	if err != nil {
		return fmt.Errorf("initialize tracing: %w", err)
	}

	otel.SetTracerProvider(tracerProvider)

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			log.Error("shutdown tracing", zap.Error(err))
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

	redisClient, err := core_cache_redis.NewClient(ctx, core_cache_redis.NewConfigMust())
	if err != nil {
		return err
	}
	defer redisClient.Close()
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

	registry := prometheus.NewRegistry()

	metrics := core_metrics.NewHTTPMetrics()
	registry.MustRegister(
		metrics.Requests,
		metrics.Duration,
	)

	r.Use(metrics.Middleware)
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
	loginLimiter, err := core_ratelimit.NewRedisLimiter(redisClient, 5, time.Second, time.Minute)
	if err != nil {
		return fmt.Errorf("create login rate limiter: %w", err)
	}
	rateLimitMetrics := core_ratelimit.NewRateLimitMetrics()
	registry.MustRegister(rateLimitMetrics.Requests)

	r.Post("/api/v1/auth/register", authTransport.Register)
	r.With(core_ratelimit.LimitByIP(loginLimiter, "ticketrush:ratelimit:login:", rateLimitMetrics)).
		Post("/api/v1/auth/login", authTransport.Login)
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
	cacheMetrics := events_cache.NewCacheMetrics()
	registry.MustRegister(cacheMetrics.Operations)
	eventCache, err := events_cache.NewEventCache(
		redisClient,
		time.Second,
		time.Minute,
		cacheMetrics,
	)
	if err != nil {
		return err
	}

	eventRepo := repository.NewEventsRepo(pool)
	eventTransport := transport.NewEventsHTTPTransport(service.NewEventsService(eventRepo, eventCache))
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

	// Booking analytics uses ClickHouse; event access is checked in PostgreSQL.
	clickhouseConfig, err := core_clickhouse.NewConfig()
	if err != nil {
		return fmt.Errorf("configure booking analytics: %w", err)
	}
	clickhouseConn, err := core_clickhouse.NewClient(ctx, clickhouseConfig)
	if err != nil {
		return fmt.Errorf("initialize booking analytics: %w", err)
	}
	defer func() {
		if err := clickhouseConn.Close(); err != nil {
			log.Error("close clickhouse", zap.Error(err))
		}
	}()
	analyticsRepo := analytics_repository.NewAnalytics(clickhouseConn, clickhouseConfig.Timeout)
	analyticsService := analytics_service.NewAnalyticsService(analyticsRepo, eventRepo)
	analyticsTransport := analytics_transport.NewHTTPAnalyticsTransport(analyticsService)
	organizerProtected.Get("/api/v1/organizer/events/{event_id}/analytics/bookings", analyticsTransport.GetBookingStats)
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

	//  ----------------------------- Booking -------------------------------------

	bookingMetrics := booking_transport.NewBookingMetrics()
	registry.MustRegister(
		bookingMetrics.Attempts,
		bookingMetrics.Duration,
	)

	workerMetrics := booking_worker.NewExpirationMetrics()
	registry.MustRegister(
		workerMetrics.Batches,
		workerMetrics.ExpiredBookings,
		workerMetrics.Duration,
		workerMetrics.LastSuccess,
	)

	bookingRepo := booking_repository.NewBookingRepository(pool)
	bookingService := booking_service.NewBookingService(bookingRepo)
	bookingTransport := booking_transport.NewHTTPBookingTransport(
		bookingService,
		bookingMetrics,
	)

	writer := &kafka.Writer{
		Addr:                   kafka.TCP("127.0.0.1:9092"),
		Topic:                  "ticketrush.bookings",
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		Async:                  false,
		BatchSize:              1,
		MaxAttempts:            3,
		ReadTimeout:            3 * time.Second,
		WriteTimeout:           3 * time.Second,
		AllowAutoTopicCreation: false,
	}
	defer func() {
		if err := writer.Close(); err != nil {
			log.Error("close kafka writer", zap.Error(err))
		}
	}()

	outboxWorker, err := outbox_worker.NewOutboxWorker(
		outbox_repository.NewOutboxRepository(pool),
		outbox_publisher.NewKafkaPublisher(writer),
		10,
		5*time.Second,
		log,
	)
	if err != nil {
		return fmt.Errorf("initialize outbox worker: %w", err)
	}

	outboxCtx, stopOutbox := context.WithCancel(ctx)
	outboxDone := make(chan struct{})

	go func() {
		defer close(outboxDone)
		outboxWorker.Run(outboxCtx)
	}()

	defer func() {
		stopOutbox()
		<-outboxDone
	}()

	r.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))
	protected.Post("/api/v1/events/{event_id}/sessions/{session_id}/bookings", bookingTransport.CreateBooking)
	protected.Get("/api/v1/bookings/{booking_id}", bookingTransport.GetBooking)
	protected.Post("/api/v1/bookings/{booking_id}/cancel", bookingTransport.CancelBooking)

	server := &http.Server{Addr: address, Handler: r, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second}
	expirationWorker, err := booking_worker.NewExpirationWorker(bookingService, log, booking_worker.DefaultExpirationConfig(), workerMetrics)
	if err != nil {
		return fmt.Errorf("initialize booking expiration: %w", err)
	}
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		expirationWorker.Run(workerCtx)
	}()
	// Stop and join the worker before deferred pool and logger closure.
	defer func() { stopWorker(); <-workerDone }()

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
