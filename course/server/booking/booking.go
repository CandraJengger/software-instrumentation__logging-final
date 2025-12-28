package booking

import (
	"context"

	"github.com/imrenagicom/demo-app/course/booking"
	"github.com/imrenagicom/demo-app/internal/instrumentation"
	v1 "github.com/imrenagicom/demo-app/pkg/apiclient/course/v1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
)

func New(svc Service) *Server {
	log.Info().Msg("initializing booking service")
	return &Server{
		service: svc,
	}
}

type Service interface {
	CreateBooking(ctx context.Context, req *v1.CreateBookingRequest) (*booking.Booking, error)
	ReserveBooking(ctx context.Context, req *v1.ReserveBookingRequest) (*booking.Booking, error)
	GetBooking(ctx context.Context, req *v1.GetBookingRequest) (*booking.Booking, error)
	ExpireBooking(ctx context.Context, req *v1.ExpireBookingRequest) (*booking.Booking, error)
	ListBookings(ctx context.Context, req *v1.ListBookingsRequest) ([]booking.Booking, string, error)
}

type Server struct {
	v1.UnimplementedBookingServiceServer

	service Service
}

func (s Server) CreateBooking(ctx context.Context, req *v1.CreateBookingRequest) (*v1.Booking, error) {
	ctx = instrumentation.LogWithContext(ctx)

	log.Ctx(ctx).Info().Msg("creating booking")

	b, err := s.service.CreateBooking(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to create booking")
		return nil, err
	}

	log.Ctx(ctx).Info().Str("booking id", b.ID.String()).Int("code", int(codes.OK)).Msg("booking created")
	return b.ApiV1(), nil
}

func (s Server) ReserveBooking(ctx context.Context, req *v1.ReserveBookingRequest) (*v1.ReserveBookingResponse, error) {
	ctx = instrumentation.LogWithContext(ctx)

	log.Ctx(ctx).Info().Str("booking id", req.GetBooking()).Msg("reserving booking")

	b, err := s.service.ReserveBooking(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to reserve booking")
		return nil, err
	}

	log.Ctx(ctx).Info().Str("booking id", req.GetBooking()).Int("code", int(codes.OK)).
		Float64("price", b.ApiV1().GetPrice()).
		Str("courseId", b.ApiV1().GetCourse()).Msg("booking reserved")
	return &v1.ReserveBookingResponse{}, nil
}

func (s Server) GetBooking(ctx context.Context, req *v1.GetBookingRequest) (*v1.Booking, error) {
	ctx = instrumentation.LogWithContext(ctx)

	log.Ctx(ctx).Info().Str("booking id", req.GetBooking()).Msg("getting booking")

	b, err := s.service.GetBooking(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to get booking")
		return nil, err
	}

	log.Ctx(ctx).Info().Str("booking id", b.ID.String()).Int("code", int(codes.OK)).Msg("booking found")
	return b.ApiV1(), nil
}

func (s Server) ExpireBooking(ctx context.Context, req *v1.ExpireBookingRequest) (*v1.ExpireBookingResponse, error) {
	ctx = instrumentation.LogWithContext(ctx)

	log.Ctx(ctx).Info().Str("booking id", req.GetBooking()).Msg("expiring booking")

	b, err := s.service.ExpireBooking(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to expire booking")
		return nil, err
	}

	log.Ctx(ctx).Info().Str("booking id", req.GetBooking()).Int("code", int(codes.OK)).
		Float64("price", b.ApiV1().GetPrice()).
		Str("courseId", b.ApiV1().GetCourse()).Msg("booking expired")
	return &v1.ExpireBookingResponse{}, nil
}

func (s Server) ListBookings(ctx context.Context, req *v1.ListBookingsRequest) (*v1.ListBookingsResponse, error) {
	ctx = instrumentation.LogWithContext(ctx)

	log.Ctx(ctx).Info().Msg("listing bookings")

	bookings, _, err := s.service.ListBookings(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to list bookings")
		return nil, err
	}
	var bks []*v1.Booking
	for _, b := range bookings {
		bks = append(bks, b.ApiV1())
	}

	log.Ctx(ctx).Info().Int("code", int(codes.OK)).Msg("bookings listed")
	return &v1.ListBookingsResponse{
		Bookings: bks,
	}, nil
}
