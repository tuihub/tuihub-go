package internal

import (
	"context"
	"time"

	sephirah "github.com/tuihub/protos/pkg/librarian/sephirah/v1"

	"github.com/go-kratos/kratos/contrib/registry/consul/v2"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	capi "github.com/hashicorp/consul/api"
)

func NewSephirahClient(ctx context.Context, config *capi.Config) (sephirah.LibrarianSephirahServiceClient, error) {
	r, err := NewRegistry(config)
	if err != nil {
		return nil, err
	}
	conn, err := grpc.DialInsecure(
		context.Background(),
		grpc.WithEndpoint("discovery:///sephirah"),
		grpc.WithDiscovery(r),
		grpc.WithMiddleware(
			recovery.Recovery(),
		),
		grpc.WithTimeout(time.Minute),
	)
	cli := sephirah.NewLibrarianSephirahServiceClient(conn)
	return cli, err
}

func NewRegistry(config *capi.Config) (*consul.Registry, error) {
	if config == nil {
		config = capi.DefaultConfig()
	}
	client, err := capi.NewClient(config)
	if err != nil {
		return nil, err
	}
	return consul.New(client), nil
}
