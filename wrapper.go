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

const (
	defaultHeartbeatInterval  = time.Second * 10
	defaultHeartbeatDowngrade = time.Second * 30
	defaultHeartbeatTimeout   = time.Second * 60
)

type serviceWrapper struct {
	pb.LibrarianPorterServiceServer
	Info         *pb.GetPorterInformationResponse
	Logger       log.Logger
	Client       sephirah.LibrarianSephirahServiceClient
	RequireToken bool
	Token        *tokenInfo

	lastHeartbeat time.Time
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
	needRefreshToken := false
	f := func() error {
		if s.Token != nil {
			if s.Token.enabler == req.GetSephirahId() {
				return nil
			} else if s.lastHeartbeat.Add(defaultHeartbeatTimeout).After(time.Now()) {
				return fmt.Errorf("porter already enabled by %d", s.Token.enabler)
			}
		}
		s.Token = new(tokenInfo)
		s.Token.enabler = req.GetSephirahId()
		s.lastHeartbeat = time.Now()
		if s.RequireToken {
			if req.GetRefreshToken() == "" {
				needRefreshToken = true
				return nil
			}
			ctx = metadata.AppendToOutgoingContext(ctx, "authorization", "Bearer "+req.GetRefreshToken())
			resp, err := s.Client.RefreshToken(ctx, new(sephirah.RefreshTokenRequest))
			if err != nil {
				return err
			}
			s.Token = &tokenInfo{
				enabler:      req.GetSephirahId(),
				AccessToken:  resp.GetAccessToken(),
				refreshToken: resp.GetRefreshToken(),
			}
		}
		return nil
	}
	if err := f(); err != nil {
		return nil, err
	}
	if resp, err := s.LibrarianPorterServiceServer.EnablePorter(ctx, req); isUnimplementedError(err) {
		return new(pb.EnablePorterResponse), nil
	} else {
		resp.NeedRefreshToken = needRefreshToken
		return resp, err
	}
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
	serviceWrapper
}

func NewService(p serviceWrapper) pb.LibrarianPorterServiceServer {
	return &service{
		p,
	}
}

func (s *service) GetPorterInformation(ctx context.Context, req *pb.GetPorterInformationRequest) (
	*pb.GetPorterInformationResponse, error) {
	return s.serviceWrapper.GetPorterInformation(ctx, req)
}
func (s *service) EnablePorter(ctx context.Context, req *pb.EnablePorterRequest) (
	*pb.EnablePorterResponse, error) {
	return s.serviceWrapper.EnablePorter(ctx, req)
}
func (s *service) PullAccount(ctx context.Context, req *pb.PullAccountRequest) (
	*pb.PullAccountResponse, error) {
	if !s.serviceWrapper.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAccountId() == nil ||
		req.GetAccountId().GetPlatform() == "" ||
		req.GetAccountId().GetPlatformAccountId() == "" {
		return nil, errors.BadRequest("Invalid account id", "")
	}
	for _, account := range s.serviceWrapper.Info.GetFeatureSummary().GetAccountPlatforms() {
		if account.GetId() == req.GetAccountId().GetPlatform() {
			return s.serviceWrapper.LibrarianPorterServiceServer.PullAccount(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported account platform", "")
}
func (s *service) PullAppInfo(ctx context.Context, req *pb.PullAppInfoRequest) (*pb.PullAppInfoResponse, error) {
	if !s.serviceWrapper.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAppInfoId() == nil ||
		req.GetAppInfoId().GetInternal() ||
		req.GetAppInfoId().GetSource() == "" ||
		req.GetAppInfoId().GetSourceAppId() == "" {
		return nil, errors.BadRequest("Invalid app id", "")
	}
	for _, source := range s.serviceWrapper.Info.GetFeatureSummary().GetAppInfoSources() {
		if source.GetId() == req.GetAppInfoId().GetSource() {
			return s.serviceWrapper.LibrarianPorterServiceServer.PullAppInfo(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported app source", "")
}
func (s *service) PullAccountAppInfoRelation(ctx context.Context, req *pb.PullAccountAppInfoRelationRequest) (
	*pb.PullAccountAppInfoRelationResponse, error) {
	if !s.serviceWrapper.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetAccountId() == nil ||
		req.GetRelationType() == librarian.AccountAppRelationType_ACCOUNT_APP_RELATION_TYPE_UNSPECIFIED ||
		req.GetAccountId().GetPlatform() == "" || req.GetAccountId().GetPlatformAccountId() == "" {
		return nil, errors.BadRequest("Invalid account id", "")
	}
	for _, account := range s.serviceWrapper.Info.GetFeatureSummary().GetAccountPlatforms() {
		if account.GetId() == req.GetAccountId().GetPlatform() {
			return s.serviceWrapper.LibrarianPorterServiceServer.PullAccountAppInfoRelation(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported account", "")
}
func (s *service) SearchAppInfo(ctx context.Context, req *pb.SearchAppInfoRequest) (*pb.SearchAppInfoResponse, error) {
	if !s.serviceWrapper.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	if req.GetName() == "" {
		return nil, errors.BadRequest("Invalid app name", "")
	}
	if len(s.serviceWrapper.Info.GetFeatureSummary().GetAppInfoSources()) > 0 {
		return s.serviceWrapper.LibrarianPorterServiceServer.SearchAppInfo(ctx, req)
	}
	return nil, errors.BadRequest("Unsupported app source", "")
}
func (s *service) PullFeed(ctx context.Context, req *pb.PullFeedRequest) (*pb.PullFeedResponse, error) {
	if !s.serviceWrapper.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	for _, source := range s.serviceWrapper.Info.GetFeatureSummary().GetFeedSources() {
		if source.GetId() == req.GetSource().GetId() {
			return s.serviceWrapper.LibrarianPorterServiceServer.PullFeed(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported feed source", "")
}
func (s *service) PushFeedItems(ctx context.Context, req *pb.PushFeedItemsRequest) (
	*pb.PushFeedItemsResponse, error) {
	if !s.serviceWrapper.Enabled() {
		return nil, errors.Forbidden("Unauthorized caller", "")
	}
	for _, destination := range s.serviceWrapper.Info.GetFeatureSummary().GetNotifyDestinations() {
		if destination.GetId() == req.GetDestination().GetId() {
			return s.serviceWrapper.LibrarianPorterServiceServer.PushFeedItems(ctx, req)
		}
	}
	return nil, errors.BadRequest("Unsupported notify destination", "")
}
