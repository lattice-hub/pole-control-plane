package builtinplugins

import (
	"fmt"

	defaultpolicy "github.com/pole-io/pole-server/plugin/access_control/auth/policy"
	defaultuser "github.com/pole-io/pole-server/plugin/access_control/auth/user"
	ratelimittoken "github.com/pole-io/pole-server/plugin/access_control/ratelimit/token"
	whitelistip "github.com/pole-io/pole-server/plugin/access_control/whitelist/ip"
	"github.com/pole-io/pole-server/plugin/apiserver/apolloserver"
	"github.com/pole-io/pole-server/plugin/apiserver/consulserver"
	"github.com/pole-io/pole-server/plugin/apiserver/eurekaserver"
	grpcdiscover "github.com/pole-io/pole-server/plugin/apiserver/grpcserver/discover"
	"github.com/pole-io/pole-server/plugin/apiserver/httpserver"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver"
	"github.com/pole-io/pole-server/plugin/apiserver/xdsserverv3"
	cmdbmemory "github.com/pole-io/pole-server/plugin/cmdb/memory"
	cryptoaes "github.com/pole-io/pole-server/plugin/crypto/aes"
	cryptorsa "github.com/pole-io/pole-server/plugin/crypto/rsa"
	discoverlogger "github.com/pole-io/pole-server/plugin/observability/discoverevent/logger"
	discoverotel "github.com/pole-io/pole-server/plugin/observability/discoverevent/otel"
	discoverrds "github.com/pole-io/pole-server/plugin/observability/discoverevent/rds"
	historylogger "github.com/pole-io/pole-server/plugin/observability/history/logger"
	historyotel "github.com/pole-io/pole-server/plugin/observability/history/otel"
	historyrds "github.com/pole-io/pole-server/plugin/observability/history/rds"
	statislogger "github.com/pole-io/pole-server/plugin/observability/statis/logger"
	statisotel "github.com/pole-io/pole-server/plugin/observability/statis/otel"
	statisprometheus "github.com/pole-io/pole-server/plugin/observability/statis/prometheus"
	"github.com/pole-io/pole-server/plugin/service/healthchecker/heartbeat"
	"github.com/pole-io/pole-server/plugin/service/healthchecker/probe"
	mysqlstore "github.com/pole-io/pole-server/plugin/store/mysql"
	"github.com/pole-io/pole-server/pluginapi"
)

type registerFunc func(*pluginapi.Registry) error

var registrations = []struct {
	name     string
	register registerFunc
}{
	{name: "auth-user/default", register: defaultuser.Register},
	{name: "auth-policy/default", register: defaultpolicy.Register},
	{name: "rate-limit/token-bucket", register: ratelimittoken.Register},
	{name: "whitelist/ip", register: whitelistip.Register},
	{name: "api-server/apollo", register: apolloserver.Register},
	{name: "api-server/consul", register: consulserver.Register},
	{name: "api-server/eureka", register: eurekaserver.Register},
	{name: "api-server/grpc", register: grpcdiscover.Register},
	{name: "api-server/http", register: httpserver.Register},
	{name: "api-server/nacos", register: nacosserver.Register},
	{name: "api-server/xds-v3", register: xdsserverv3.Register},
	{name: "cmdb/memory", register: cmdbmemory.Register},
	{name: "crypto/aes", register: cryptoaes.Register},
	{name: "crypto/rsa", register: cryptorsa.Register},
	{name: "discover-event/logger", register: discoverlogger.Register},
	{name: "discover-event/otel", register: discoverotel.Register},
	{name: "discover-event/rds", register: discoverrds.Register},
	{name: "history/logger", register: historylogger.Register},
	{name: "history/otel", register: historyotel.Register},
	{name: "history/rds", register: historyrds.Register},
	{name: "statis/logger", register: statislogger.Register},
	{name: "statis/otel", register: statisotel.Register},
	{name: "statis/prometheus", register: statisprometheus.Register},
	{name: "health-check/heartbeat", register: heartbeat.Register},
	{name: "health-check/probe", register: probe.Register},
	{name: "store/mysql", register: mysqlstore.Register},
}

func Register(registry *pluginapi.Registry) error {
	for _, entry := range registrations {
		if err := entry.register(registry); err != nil {
			return fmt.Errorf("register builtin plugin %s: %w", entry.name, err)
		}
	}
	return nil
}

func MustRegister(registry *pluginapi.Registry) {
	if err := Register(registry); err != nil {
		panic(err)
	}
}

func NewRegistry() (*pluginapi.Registry, error) {
	registry := pluginapi.NewRegistry()
	if err := Register(registry); err != nil {
		return nil, err
	}
	return registry, nil
}
