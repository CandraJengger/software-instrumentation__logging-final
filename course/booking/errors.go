package booking

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrReservationMaxRetryExceeded = errors.New("reservation max retry exceeded")
	ErrReleaseMaxRetryExceeded     = errors.New("booking release max retry exceeded")

	ErrBookingAlreadyExpired   = errors.New("booking already expired")
	ErrBookingAlreadyCompleted = ErrInvalidStateChange{Message: "booking already completed"}
)

type ErrInvalidStateChange struct {
	Context context.Context
	Message string
}

func (e ErrInvalidStateChange) Error() string {
	return e.Message
}

func (e ErrInvalidStateChange) GRPCStatus() *status.Status {
	log.Ctx(e.Context).Error().Int("code", int(codes.FailedPrecondition)).Msg(e.Message)
	return status.New(codes.FailedPrecondition, e.Error())
}
