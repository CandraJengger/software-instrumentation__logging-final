package db

import (
	"context"
	"errors"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	ErrNoRowUpdated = errors.New("no row is updated")
)

type ErrResourceNotFound struct {
	Context context.Context
	Message string
}

func (e ErrResourceNotFound) Error() string {
	return e.Message
}

func (e ErrResourceNotFound) GRPCStatus() *status.Status {
	log.Ctx(e.Context).Error().Int("code", int(codes.NotFound)).Msg(e.Message)
	return status.New(codes.NotFound, e.Error())
}

type ErrInvalidUuid struct {
	Context context.Context
	Message string
}

func (e ErrInvalidUuid) Error() string {
	return e.Message
}

func (e ErrInvalidUuid) GRPCStatus() *status.Status {
	log.Ctx(e.Context).Error().Int("code", int(codes.InvalidArgument)).Msg(e.Message)
	return status.New(codes.InvalidArgument, e.Error())
}
