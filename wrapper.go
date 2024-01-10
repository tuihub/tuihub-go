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

type wrapper struct {
	Handler Handler
	Config  PorterConfig
	Logger  log.Logger
	Token   *tokenInfo
	Client  sephirah.LibrarianSephirahServiceClient
}

type tokenInfo struct {
	enabler      int64
	AccessToken  string
	refreshToken string
}

func (s *wrapper) GetPorterInformation(ctx context.Context, req *pb.GetPorterInformationRequest) (
	*pb.GetPorterInformationResponse, error) {
	return &pb.GetPorterInformationResponse{
		Name:           s.Config.Name,
		Version:        s.Config.Version,
		GlobalName:     s.Config.GlobalName,
		FeatureSummary: s.Config.FeatureSummary,
	}, nil
}
func (s *wrapper) EnablePorter(ctx context.Context, req *pb.EnablePorterRequest) (
	*pb.EnablePorterResponse, error) {
	if s.Token != nil {
		if s.Token.enabler == req.GetSephirahId() {
			return &pb.EnablePorterResponse{}, nil
		} else {
			return nil, fmt.Errorf("porter already enabled by %d", s.Token.enabler)
		}
	}
	ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+req.GetRefreshToken())
	resp, err := s.Client.RefreshToken(ctx, &sephirah.RefreshTokenRequest{})
	if err != nil {
		return nil, err
	}
	s.Token = &tokenInfo{
		enabler:      req.GetSephirahId(),
		AccessToken:  resp.GetAccessToken(),
		refreshToken: resp.GetRefreshToken(),
	}
	return &pb.EnablePorterResponse{}, nil
}
func (s *wrapper) Enabled() bool {
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
	p wrapper
}

func NewService(p wrapper) pb.LibrarianPorterServiceServer {
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
	for _, account := range s.p.Config.FeatureSummary.GetSupportedAccounts() {
		if account.GetPlatform() == req.GetAccountId().GetPlatform() {
			return s.p.Handler.PullAccount(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported account platform", "")
}
func (s *service) PullApp(ctx context.Context, req *pb.PullAppRequest) (*pb.PullAppResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAppId() == nil ||
		req.GetAppId().GetInternal() ||
		req.GetAppId().GetSource() == "" ||
		req.GetAppId().GetSourceAppId() == "" {
		return nil, errors.BadRequest("Invalid app id", "")
	}
	for _, source := range s.p.Config.FeatureSummary.GetSupportedAppSources() {
		if source == req.GetAppId().GetSource() {
			return s.p.Handler.PullApp(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported app source", "")
}
func (s *service) PullAccountAppRelation(ctx context.Context, req *pb.PullAccountAppRelationRequest) (
	*pb.PullAccountAppRelationResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAccountId() == nil ||
		req.GetRelationType() == librarian.AccountAppRelationType_ACCOUNT_APP_RELATION_TYPE_UNSPECIFIED ||
		req.GetAccountId().GetPlatform() == "" || req.GetAccountId().GetPlatformAccountId() == "" {
		return nil, errors.BadRequest("Invalid account id", "")
	}
	for _, account := range s.p.Config.FeatureSummary.GetSupportedAccounts() {
		if account.GetPlatform() == req.GetAccountId().GetPlatform() {
			for _, relationType := range account.GetAppRelationTypes() {
				if relationType == req.GetRelationType() {
					return s.p.Handler.PullAccountAppRelation(ctx, req)
				}
			}
			return nil, errors.BadRequest("Unsupported relation type", "")
		}
	}
	return nil, errors.BadRequest("Unsupported account", "")
}
func (s *service) PullFeed(ctx context.Context, req *pb.PullFeedRequest) (*pb.PullFeedResponse, error) {
	if !s.p.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetSource() == "" ||
		req.GetChannelId() == "" {
		return nil, errors.BadRequest("Invalid feed id", "")
	}
	for _, source := range s.p.Config.FeatureSummary.GetSupportedFeedSources() {
		if source == req.GetSource() {
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
	if req.GetDestination() == "" || req.GetChannelId() == "" || len(req.GetItems()) == 0 {
		return nil, errors.BadRequest("Invalid feed id", "")
	}
	for _, destination := range s.p.Config.FeatureSummary.GetSupportedNotifyDestinations() {
		if destination == req.GetDestination() {
			return s.p.Handler.PushFeedItems(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported notify destination", "")
}
