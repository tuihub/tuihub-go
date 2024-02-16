package tuihub

import (
	"context"

	pb "github.com/tuihub/protos/pkg/librarian/porter/v1"
)

type Handler interface {
	PullAccount(context.Context, *pb.PullAccountRequest) (*pb.PullAccountResponse, error)
	PullAppInfo(context.Context, *pb.PullAppInfoRequest) (*pb.PullAppInfoResponse, error)
	PullAccountAppInfoRelation(
		context.Context, *pb.PullAccountAppInfoRelationRequest,
	) (*pb.PullAccountAppInfoRelationResponse, error)
	SearchAppInfo(context.Context, *pb.SearchAppInfoRequest) (*pb.SearchAppInfoResponse, error)
	PullFeed(context.Context, *pb.PullFeedRequest) (*pb.PullFeedResponse, error)
	PushFeedItems(context.Context, *pb.PushFeedItemsRequest) (*pb.PushFeedItemsResponse, error)
}
