package tuihub

import (
	"context"
	"fmt"
	"time"

	pb "github.com/tuihub/protos/pkg/librarian/porter/v1"
	sephirah "github.com/tuihub/protos/pkg/librarian/sephirah/v1"
	librarian "github.com/tuihub/protos/pkg/librarian/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/middleware"
	"github.com/go-kratos/kratos/v2/middleware/logging"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc/metadata"
)

type serviceWrapper struct {
	Handler      Handler
	Info         *pb.GetPorterInformationResponse
	Logger       log.Logger
	Client       sephirah.LibrarianSephirahServiceClient
	RequireToken bool
	Token        *tokenInfo
}

type tokenInfo struct {
	enabler      int64
	AccessToken  string
	refreshToken string
}

func (s *serviceWrapper) GetPorterInformation(ctx context.Context, req *pb.GetPorterInformationRequest) (
	*pb.GetPorterInformationResponse, error) {
	return s.Info, nil
}
func (s *serviceWrapper) EnablePorter(ctx context.Context, req *pb.EnablePorterRequest) (
	*pb.EnablePorterResponse, error) {
	if s.Token != nil {
		if s.Token.enabler == req.GetSephirahId() {
			return &pb.EnablePorterResponse{
				StatusMessage:    "",
				NeedRefreshToken: false,
				EnablesSummary:   nil,
			}, nil
		} else {
			return nil, fmt.Errorf("porter already enabled by %d", s.Token.enabler)
		}
	}
	if s.RequireToken {
		ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+req.GetRefreshToken())
		resp, err := s.Client.RefreshToken(ctx, new(sephirah.RefreshTokenRequest))
		if err == nil {
			s.Token = &tokenInfo{
				enabler:      req.GetSephirahId(),
				AccessToken:  resp.GetAccessToken(),
				refreshToken: resp.GetRefreshToken(),
			}
			return &pb.EnablePorterResponse{
				StatusMessage:    "",
				NeedRefreshToken: false,
				EnablesSummary:   nil,
			}, nil
		}
	}
	s.Token = new(tokenInfo)
	s.Token.enabler = req.GetSephirahId()
	return &pb.EnablePorterResponse{
		StatusMessage:    "",
		NeedRefreshToken: true,
		EnablesSummary:   nil,
	}, nil
}
func (s *serviceWrapper) Enabled() bool {
	return s.Token != nil
}

func NewServer(c *ServerConfig, service pb.LibrarianPorterServiceServer, logger log.Logger) *grpc.Server {
	var middlewares = []middleware.Middleware{
		logging.Server(logger),
	}
	var opts = []grpc.ServerOption{
		grpc.Middleware(middlewares...),
	}
	if c.Network != "" {
		opts = append(opts, grpc.Network(c.Network))
	}
	if c.Addr != "" {
		opts = append(opts, grpc.Address(c.Addr))
	}
	if c.Timeout != nil {
		opts = append(opts, grpc.Timeout(*c.Timeout))
	} else {
		opts = append(opts, grpc.Timeout(time.Minute))
	}
	srv := grpc.NewServer(opts...)
	pb.RegisterLibrarianPorterServiceServer(srv, service)
	return srv
}

type service struct {
	pb.UnimplementedLibrarianPorterServiceServer
	p serviceWrapper
}

func NewService(p serviceWrapper) pb.LibrarianPorterServiceServer {
	return &service{
		UnimplementedLibrarianPorterServiceServer: pb.UnimplementedLibrarianPorterServiceServer{},
		p: p,
	}
}

func (s *service) GetPorterInformation(ctx context.Context, req *pb.GetPorterInformationRequest) (
	*pb.GetPorterInformationResponse, error) {
	return s.p.GetPorterInformation(ctx, req)
}
func (s *service) EnablePorter(ctx context.Context, req *pb.EnablePorterRequest) (
	*pb.EnablePorterResponse, error) {
	return s.p.EnablePorter(ctx, req)
}
func (s *service) PullAccount(ctx context.Context, req *pb.PullAccountRequest) (
	*pb.PullAccountResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAccountId() == nil ||
		req.GetAccountId().GetPlatform() == "" ||
		req.GetAccountId().GetPlatformAccountId() == "" {
		return nil, errors.BadRequest("Invalid account id", "")
	}
	for _, account := range s.p.Info.GetFeatureSummary().GetAccountPlatforms() {
		if account.GetId() == req.GetAccountId().GetPlatform() {
			return s.p.Handler.PullAccount(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported account platform", "")
}
func (s *service) PullAppInfo(ctx context.Context, req *pb.PullAppInfoRequest) (*pb.PullAppInfoResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAppInfoId() == nil ||
		req.GetAppInfoId().GetInternal() ||
		req.GetAppInfoId().GetSource() == "" ||
		req.GetAppInfoId().GetSourceAppId() == "" {
		return nil, errors.BadRequest("Invalid app id", "")
	}
	for _, source := range s.p.Info.GetFeatureSummary().GetAppInfoSources() {
		if source.GetId() == req.GetAppInfoId().GetSource() {
			return s.p.Handler.PullAppInfo(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported app source", "")
}
func (s *service) PullAccountAppInfoRelation(ctx context.Context, req *pb.PullAccountAppInfoRelationRequest) (
	*pb.PullAccountAppInfoRelationResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAccountId() == nil ||
		req.GetRelationType() == librarian.AccountAppRelationType_ACCOUNT_APP_RELATION_TYPE_UNSPECIFIED ||
		req.GetAccountId().GetPlatform() == "" || req.GetAccountId().GetPlatformAccountId() == "" {
		return nil, errors.BadRequest("Invalid account id", "")
	}
	for _, account := range s.p.Info.GetFeatureSummary().GetAccountPlatforms() {
		if account.GetId() == req.GetAccountId().GetPlatform() {
			return s.p.Handler.PullAccountAppInfoRelation(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported account", "")
}
func (s *service) SearchAppInfo(ctx context.Context, req *pb.SearchAppInfoRequest) (*pb.SearchAppInfoResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetName() == "" {
		return nil, errors.BadRequest("Invalid app name", "")
	}
	if len(s.p.Info.GetFeatureSummary().GetAppInfoSources()) > 0 {
		return s.p.Handler.SearchAppInfo(ctx, req)
	}
	return nil, errors.BadRequest("Unsupported app source", "")
}
func (s *service) PullFeed(ctx context.Context, req *pb.PullFeedRequest) (*pb.PullFeedResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	for _, source := range s.p.Info.GetFeatureSummary().GetFeedSources() {
		if source.GetId() == req.GetSource().GetId() {
			return s.p.Handler.PullFeed(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported feed source", "")
}
func (s *service) PushFeedItems(ctx context.Context, req *pb.PushFeedItemsRequest) (
	*pb.PushFeedItemsResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	for _, destination := range s.p.Info.GetFeatureSummary().GetNotifyDestinations() {
		if destination.GetId() == req.GetDestination().GetId() {
			return s.p.Handler.PushFeedItems(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported notify destination", "")
}
