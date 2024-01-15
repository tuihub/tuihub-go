package tuihub

import (
	"context"

	pb "github.com/tuihub/protos/pkg/librarian/porter/v1"
)

type Handler interface {
	PullAccount(context.Context, *pb.PullAccountRequest) (*pb.PullAccountResponse, error)
	PullApp(context.Context, *pb.PullAppRequest) (*pb.PullAppResponse, error)
	PullAccountAppRelation(context.Context, *pb.PullAccountAppRelationRequest) (*pb.PullAccountAppRelationResponse, error)
	SearchApp(context.Context, *pb.SearchAppRequest) (*pb.SearchAppResponse, error)
	PullFeed(context.Context, *pb.PullFeedRequest) (*pb.PullFeedResponse, error)
	PushFeedItems(context.Context, *pb.PushFeedItemsRequest) (*pb.PushFeedItemsResponse, error)
}
