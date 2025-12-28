package catalog

import (
	"context"

	"github.com/imrenagicom/demo-app/course/catalog"
	"github.com/imrenagicom/demo-app/internal/instrumentation"
	v1 "github.com/imrenagicom/demo-app/pkg/apiclient/course/v1"
	"github.com/rs/zerolog/log"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Service interface {
	ListCourse(ctx context.Context, req *v1.ListCoursesRequest) ([]catalog.Course, string, error)
	GetCourse(ctx context.Context, req *v1.GetCourseRequest) (*catalog.Course, error)
}

func New(s Service) *Server {
	log.Info().Msg("initializing catalog server")
	return &Server{
		service: s,
	}
}

type Server struct {
	v1.UnimplementedCatalogServiceServer

	service Service
}

func (s Server) ListCourses(ctx context.Context, req *v1.ListCoursesRequest) (*v1.ListCoursesResponse, error) {
	ctx = instrumentation.LogWithContext(ctx)
	courses, nextPage, err := s.service.ListCourse(ctx, req)
	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to list courses")
		return nil, status.New(codes.FailedPrecondition, err.Error()).Err()
	}

	var data []*v1.Course
	for _, c := range courses {
		data = append(data, c.ApiV1())
	}

	res := &v1.ListCoursesResponse{
		Courses:       data,
		NextPageToken: nextPage,
	}
	log.Ctx(ctx).Info().Int("code", int(codes.OK)).Msg("courses listed")
	return res, nil
}

func (s Server) GetCourse(ctx context.Context, req *v1.GetCourseRequest) (*v1.Course, error) {
	ctx = instrumentation.LogWithContext(ctx)

	log.Ctx(ctx).Info().Str("course id", req.GetCourse()).Msg("getting course")

	course, err := s.service.GetCourse(ctx, req)

	if err != nil {
		log.Ctx(ctx).Error().Err(err).Msg("unable to get course")
		return nil, err
	}

	log.Ctx(ctx).Info().Str("course id", course.ID.String()).Int("code", int(codes.OK)).Msg("course found")
	return course.ApiV1(), nil
}
