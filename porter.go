package tuihub

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	porter "github.com/tuihub/protos/pkg/librarian/porter/v1"
	sephirah "github.com/tuihub/protos/pkg/librarian/sephirah/v1"
	librarian "github.com/tuihub/protos/pkg/librarian/v1"
	"github.com/tuihub/tuihub-go/internal"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/log"
	"github.com/go-kratos/kratos/v2/transport/grpc"
	capi "github.com/hashicorp/consul/api"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	serverNetwork = "SERVER_NETWORK"
	serverPort    = "SERVER_PORT"
	serverTimeout = "SERVER_TIMEOUT"
)

type Porter struct {
	server       *grpc.Server
	wrapper      wrapper
	logger       log.Logger
	app          *kratos.App
	consulConfig *capi.Config
	serverConfig ServerConfig
}

type PorterConfig struct {
	Name           string
	Version        string
	GlobalName     string
	FeatureSummary *porter.PorterFeatureSummary
	Server         *ServerConfig
}

type ServerConfig struct {
	Network string
	Addr    string
	Timeout *time.Duration
}

type PorterOption func(*Porter)

func WithLogger(logger log.Logger) PorterOption {
	return func(p *Porter) {
		p.logger = logger
	}
}

func WithPorterConsulConfig(config *capi.Config) PorterOption {
	return func(p *Porter) {
		p.consulConfig = config
	}
}

func (p *Porter) Run() error {
	return p.app.Run()
}

func (p *Porter) Stop() error {
	return p.app.Stop()
}

func NewPorter(ctx context.Context, config PorterConfig, handler Handler, options ...PorterOption) (*Porter, error) {
	if handler == nil {
		return nil, fmt.Errorf("handler is nil")
	}
	p := new(Porter)
	p.logger = log.DefaultLogger
	for _, o := range options {
		o(p)
	}
	if p.consulConfig == nil {
		p.serverConfig = defaultServerConfig()
	}
	client, err := internal.NewSephirahClient(ctx, p.consulConfig)
	if err != nil {
		return nil, err
	}
	r, err := internal.NewRegistry(p.consulConfig)
	if err != nil {
		return nil, err
	}
	c := wrapper{
		Handler: handler,
		Config:  config,
		Logger:  p.logger,
		Token:   nil,
		Client:  client,
	}
	p.wrapper = c
	p.server = NewServer(
		config.Server,
		NewService(c),
		p.logger,
	)
	id, _ := os.Hostname()
	name := "porter"
	app := kratos.New(
		kratos.ID(id+name),
		kratos.Name(name),
		kratos.Version(p.wrapper.Config.Version),
		kratos.Metadata(map[string]string{
			"PorterName": p.wrapper.Config.GlobalName,
		}),
		kratos.Server(p.server),
		kratos.Registrar(r),
	)
	p.app = app
	return p, nil
}

func defaultServerConfig() ServerConfig {
	minute := time.Minute
	config := ServerConfig{
		Network: "0.0.0.0",
		Addr:    "",
		Timeout: &minute,
	}
	if network, exist := os.LookupEnv(serverNetwork); exist {
		config.Network = network
	}
	if port, exist := os.LookupEnv(serverPort); exist {
		config.Addr = port
	}
	if timeout, exist := os.LookupEnv(serverTimeout); exist {
		d, err := time.ParseDuration(timeout)
		if err == nil {
			config.Timeout = &d
		}
	}
	return config
}

func WellKnownToString(e protoreflect.Enum) string {
	return fmt.Sprint(proto.GetExtension(
		e.
			Descriptor().
			Values().
			ByNumber(
				e.
					Number(),
			).
			Options(),
		librarian.E_ToString,
	))
}

func (p *Porter) AsUser(ctx context.Context, userID int64) (*LibrarianClient, error) {
	if p.wrapper.Token == nil {
		return nil, fmt.Errorf("porter not enabled")
	}
	client, err := internal.NewSephirahClient(ctx, p.consulConfig)
	if err != nil {
		return nil, err
	}
	resp, err := client.GainUserPrivilege(
		WithToken(ctx, p.wrapper.Token.AccessToken),
		&sephirah.GainUserPrivilegeRequest{
			UserId: &librarian.InternalID{Id: userID},
		},
	)
	if err != nil {
		return nil, err
	}
	return &LibrarianClient{
		LibrarianSephirahServiceClient: client,
		accessToken:                    resp.GetAccessToken(),
		refreshToken:                   "",
		muToken:                        sync.RWMutex{},
		backgroundRefresh:              false,
		consulConfig:                   p.consulConfig,
	}, nil
}
