package apiserver

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	_ "net/http/pprof"

	"github.com/google/uuid"
	"github.com/imrenagicom/demo-app/course/booking"
	"github.com/imrenagicom/demo-app/course/catalog"
	bookingsrv "github.com/imrenagicom/demo-app/course/server/booking"
	catalogsrv "github.com/imrenagicom/demo-app/course/server/catalog"
	"github.com/imrenagicom/demo-app/internal/config"
	"github.com/imrenagicom/demo-app/internal/util"
	v1 "github.com/imrenagicom/demo-app/pkg/apiclient/course/v1"

	"github.com/gorilla/mux"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

var serviceTelemetryName = "course-service"

type ServerOpts struct {
	Clients *util.Clients
	Config  config.Server
}

func NewServer(opts ServerOpts) Server {
	log.Debug().
		Str("postgres", fmt.Sprintf("%s:%s/%s", opts.Config.DB.Host, opts.Config.DB.Port, opts.Config.DB.Name)).
		Str("redis", opts.Config.Redis.Addr()).
		Msg("checking config")

	s := Server{
		opts:    opts,
		clients: opts.Clients,
	}

	s.catalogStore = catalog.NewStore(opts.Clients.DB, opts.Clients.Redis)
	s.catalogService = catalog.NewService(s.catalogStore, opts.Clients.DB)
	s.bookingStore = booking.NewStore(opts.Clients.DB, opts.Clients.Redis)
	s.bookingService = booking.NewService(
		opts.Clients.DB,
		s.bookingStore,
		s.catalogStore,
	)
	return s
}

type Server struct {
	opts                 ServerOpts
	clients              *util.Clients
	otlpCollectorAddress string

	bookingService *booking.Service
	bookingStore   *booking.Store
	catalogService *catalog.Service
	catalogStore   *catalog.Store
}

// Run runs the gRPC-Gateway, dialing the provided address.
func (s *Server) Run(ctx context.Context) error {
	log.Info().Msg("starting server")

	grpcServer := s.newGRPCServer(ctx)
	go func() {
		log.Info().Msgf("initializing grpc server on %s", s.opts.Config.GRPC.Addr())
		lis, err := net.Listen("tcp", s.opts.Config.GRPC.Addr())
		if err != nil {
			log.Fatal().Msgf("failed to listen: %v", err)
		}
		log.Info().Msgf("starting grpc server on %s", s.opts.Config.GRPC.Addr())
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatal().Err(err).Msg("unable to start grpc server")
		}
	}()

	httpServer := s.newHTTPServer(ctx)
	go func() {
		log.Info().Msgf("Starting http server for serving gRPC-Gateway and OpenAPI Documentation on %s", s.opts.Config.HTTP.Addr())
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal().Err(err).Msgf("listen:%+s\n", err)
		}
	}()

	<-ctx.Done()

	gracefulShutdownPeriod := 30 * time.Second

	log.Warn().Msg("shutting down http server")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), gracefulShutdownPeriod)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("failed to shutdown http server gracefully")
	}
	log.Warn().Msg("http server gracefully stopped")

	log.Warn().Msg("shutting down grpc server")
	grpcServer.GracefulStop()
	log.Warn().Msg("grpc server gracefully stopped")

	log.Warn().Msg("clean up storage")
	if err := s.catalogStore.Clear(); err != nil {
		log.Warn().Err(err).Msg("failed to clear concert store")
	}
	if err := s.bookingStore.Clear(); err != nil {
		log.Warn().Err(err).Msg("failed to clear concert store")
	}
	return nil
}

func (s *Server) newGRPCServer(ctx context.Context) *grpc.Server {
	opts := []grpc.ServerOption{}
	grpcServer := grpc.NewServer(opts...)
	bookingSrv := bookingsrv.New(s.bookingService)
	catalogSrv := catalogsrv.New(s.catalogService)
	v1.RegisterBookingServiceServer(grpcServer, bookingSrv)
	v1.RegisterCatalogServiceServer(grpcServer, catalogSrv)
	return grpcServer
}

func (s *Server) newHTTPServer(ctx context.Context) *http.Server {
	gRPCEndpoint := s.opts.Config.GRPC.Addr()
	conn, err := grpc.DialContext(
		ctx,
		gRPCEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatal().Err(err).Msgf("failed to dial grpc server: %v", err)
	}

	gwmux := runtime.NewServeMux(
		runtime.WithMetadata(func(ctx context.Context, r *http.Request) metadata.MD {
			md := make(map[string]string)
			if method, ok := runtime.RPCMethod(ctx); ok {
				md["method"] = method // /grpc.gateway.examples.internal.proto.examplepb.LoginService/Login
			}
			if pattern, ok := runtime.HTTPPathPattern(ctx); ok {
				md["pattern"] = pattern // /v1/example/login
			}

			requestID := r.Header.Get("request_id")

			r.Header.Set("grpc_method", md["method"])

			log.Info().Str("request_id", requestID).Str("grpc_method", md["method"]).Msg("RPC Method")
			md["request_id"] = requestID
			return metadata.New(md)
		}),
	)

	mux := mux.NewRouter()
	mux.HandleFunc("/healthz", s.healthz())
	mux.HandleFunc("/readyz", s.readyz())

	mux.PathPrefix("/debug/").Handler(http.DefaultServeMux)

	api := mux.PathPrefix("/api/course").Subrouter()
	api.Use(logMiddleware) // TODO add required middleware for /api here
	api.PathPrefix("/v1").Handler(gwmux)

	mustRegisterGWHandler(ctx, v1.RegisterCatalogServiceHandler, gwmux, conn)
	mustRegisterGWHandler(ctx, v1.RegisterBookingServiceHandler, gwmux, conn)

	sh := http.StripPrefix("/swagger/",
		http.FileServer(http.Dir("./third_party/OpenAPI/")))
	mux.PathPrefix("/swagger/").Handler(sh)

	gwServer := &http.Server{
		Addr:    s.opts.Config.HTTP.Addr(),
		Handler: mux,
	}
	return gwServer
}

type registerFunc func(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error

// mustRegisterGWHandler is a convenience function to register a gateway handler.
func mustRegisterGWHandler(ctx context.Context, register registerFunc, mux *runtime.ServeMux, conn *grpc.ClientConn) {
	err := register(ctx, mux, conn)
	if err != nil {
		panic(err)
	}
}

func (s *Server) healthz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func (s *Server) readyz() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}
}

func logMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.New().String()
		start := time.Now()

		r.Header.Set("request_id", requestID)
		lrw := newLoggingResponseWriter(w)
		log.Info().
			Str("request_id", requestID).
			Str("url", r.URL.String()).
			Str("method", r.Method).
			Str("IP", r.RemoteAddr).
			Msg("request received")

		next.ServeHTTP(lrw, r)
		duration := time.Since(start).Milliseconds()
		// 🔥 Log AFTER response is written
		log.Info().
			Str("request_id", requestID).
			Str("url", r.URL.String()).
			Str("method", r.Method).
			Str("grpc_method", r.Header.Get("grpc_method")).
			Int("status", lrw.statusCode).
			Int64("duration_ms", duration).
			Msg("request completed")
	})
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{
		ResponseWriter: w,
		statusCode:     http.StatusOK, // default
	}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
