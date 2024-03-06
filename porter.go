package tuihub

import (
	"context"
	"errors"
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
	serverNetwork       = "SERVER_NETWORK"
	serverAddr          = "SERVER_ADDRESS"
	serverTimeout       = "SERVER_TIMEOUT"
	consulAddr          = "CONSUL_ADDRESS"
	consulToken         = "CONSUL_TOKEN"
	sephirahServiceName = "SEPHIRAH_SERVICE_NAME"
)

type Porter struct {
	server       *grpc.Server
	wrapper      wrapper
	logger       log.Logger
	app          *kratos.App
	consulConfig *capi.Config
	serverConfig *ServerConfig
}

type PorterConfig struct {
	Name           string
	Version        string
	GlobalName     string
	FeatureSummary *porter.PorterFeatureSummary
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
		return nil, errors.New("handler is nil")
	}
	if config.GlobalName == "" {
		return nil, errors.New("global name is empty")
	}
	if config.FeatureSummary == nil {
		return nil, errors.New("feature summary is nil")
	}
	p := new(Porter)
	p.logger = log.DefaultLogger
	for _, o := range options {
		o(p)
	}
	if p.serverConfig == nil {
		p.serverConfig = defaultServerConfig()
	}
	if p.consulConfig == nil {
		p.consulConfig = defaultConsulConfig()
	}
	client, err := internal.NewSephirahClient(ctx, p.consulConfig, os.Getenv(sephirahServiceName))
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
		p.serverConfig,
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

func defaultServerConfig() *ServerConfig {
	config := ServerConfig{
		Network: "",
		Addr:    "",
		Timeout: nil,
	}
	if network, exist := os.LookupEnv(serverNetwork); exist {
		config.Network = network
	}
	if addr, exist := os.LookupEnv(serverAddr); exist {
		config.Addr = addr
	}
	if timeout, exist := os.LookupEnv(serverTimeout); exist {
		d, err := time.ParseDuration(timeout)
		if err == nil {
			config.Timeout = &d
		}
	}
	return &config
}

func defaultConsulConfig() *capi.Config {
	config := capi.DefaultConfig()
	if addr, exist := os.LookupEnv(consulAddr); exist {
		config.Address = addr
	}
	if token, exist := os.LookupEnv(consulToken); exist {
		config.Token = token
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
		return nil, errors.New("porter not enabled")
	}
	client, err := internal.NewSephirahClient(ctx, p.consulConfig, os.Getenv(sephirahServiceName))
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
