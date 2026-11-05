package apolloserver

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/access_control/auth"
	"github.com/pole-io/pole-server/apis/access_control/ratelimit"
	"github.com/pole-io/pole-server/apis/apiserver"
	"github.com/pole-io/pole-server/apis/observability/statis"
	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/pkg/types/metrics"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	v1 "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/conn/keepalive"
	connlimit "github.com/pole-io/pole-server/pkg/common/conn/limit"
	"github.com/pole-io/pole-server/pkg/common/utils"
	httputils "github.com/pole-io/pole-server/pkg/common/utils/http"
	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
	"github.com/pole-io/pole-server/pkg/config"
	"github.com/pole-io/pole-server/pkg/service"
)

var (
	Err_NotFoundConfig = errors.New("not found config file, please check your appId, cluster and namespace")
)

// ApolloServer is the Apollo server
type ApolloServer struct {
	server          *http.Server
	connLimitConfig *connlimit.Config
	option          map[string]interface{}
	openAPI         map[string]apiserver.APIConfig

	policySvr   auth.StrategyServer
	userSvr     auth.UserServer
	discoverSvr service.DiscoverServer
	configSvr   compatconfig.Discoverer

	listenPort uint32
	listenIP   string

	watchTimeOut time.Duration

	// 环境服务器配置
	metaSvrs *svctypes.ServiceKey
}

// GetProtocol API协议名
func (a *ApolloServer) GetProtocol() string {
	return Protocol
}

// GetPort API的监听端口
func (a *ApolloServer) GetPort() uint32 {
	return a.listenPort
}

// Initialize API初始化逻辑
func (a *ApolloServer) Initialize(ctx context.Context, option map[string]interface{}, api map[string]apiserver.APIConfig) error {
	if ipValue, ok := option[optionListenIP]; ok {
		a.listenIP = ipValue.(string)
	} else {
		a.listenIP = DefaultListenIP
	}
	if portValue, ok := option[optionListenPort]; ok {
		a.listenPort = uint32(portValue.(int))
	} else {
		a.listenPort = uint32(DefaultListenPort)
	}
	a.option = option
	a.openAPI = api

	// 连接数限制的配置
	if raw, _ := option[optionConnLimit].(map[interface{}]interface{}); raw != nil {
		connLimitConfig, err := connlimit.ParseConnLimitConfig(raw)
		if err != nil {
			return err
		}
		a.connLimitConfig = connLimitConfig
	}

	// 解析环境服务器配置
	if raw, _ := option[optionMetaServer].(map[interface{}]interface{}); raw != nil {
		ns, _ := raw["namespace"].(string)
		svc, _ := raw["service"].(string)
		if ns == "" || svc == "" {
			return fmt.Errorf("invalid meta server config, namespace and service must be set")
		}
		a.metaSvrs = &svctypes.ServiceKey{
			Namespace: ns,
			Name:      svc,
		}
	}
	return nil
}

// Run API服务的主逻辑循环
func (a *ApolloServer) Run(errCh chan error) {
	var err error
	// 引入功能模块和插件
	a.policySvr, err = auth.GetStrategyServer()
	if err != nil {
		errCh <- err
		return
	}
	a.userSvr, err = auth.GetUserServer()
	if err != nil {
		errCh <- err
		return
	}
	a.configSvr, err = config.GetUnifiedService()
	if err != nil {
		errCh <- err
		return
	}
	a.discoverSvr, err = service.GetOriginServer()
	if err != nil {
		errCh <- err
		return
	}

	address := fmt.Sprintf("%v:%v", a.listenIP, a.listenPort)
	wsContainer, err := a.createRestfulContainer()
	if err != nil {
		errCh <- err
		return
	}

	server := http.Server{Addr: address, Handler: wsContainer, WriteTimeout: 2 * time.Minute}

	ln, err := net.Listen("tcp", address)
	if err != nil {
		errCh <- err
		return
	}
	ln = keepalive.NewTcpKeepAliveListener(3*time.Minute, ln.(*net.TCPListener))
	// 开启最大连接数限制
	if a.connLimitConfig != nil && a.connLimitConfig.OpenConnLimit {
		ln, err = connlimit.NewListener(ln, a.GetProtocol(), a.connLimitConfig)
		if err != nil {
			errCh <- err
			return
		}
	}
	a.server = &server

	// 开始对外服务
	if err = server.Serve(ln); err != nil && err != http.ErrServerClosed {
		errCh <- err
		return
	}
}

// Stop 停止API端口监听
func (a *ApolloServer) Stop() {
	// 释放connLimit的数据，如果没有开启，也需要执行一下
	// 目的：防止restart的时候，connLimit冲突
	connlimit.RemoveLimitListener(a.GetProtocol())
	// stop http server
	if a.server != nil {
		// 延迟三秒，等待http server关闭，做到流量无损。
		// 在此之前已经建立的链接，会正常执行业务，若执行时长超过3秒，则会抛出异常。
		// 若是在3秒内提前处理完所有请求，h.server会提前关闭。
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := a.server.Shutdown(ctx); nil != err {
			apollolog.Errorf("httpserver shutdown failed, err: %v\n", err)
		}
	}
}

// Restart 重启API
func (a *ApolloServer) Restart(option map[string]interface{}, api map[string]apiserver.APIConfig, errCh chan error) error {
	return nil
}

// 创建handler
func (a *ApolloServer) createRestfulContainer() (*restful.Container, error) {
	wsContainer := restful.NewContainer()
	wsContainer.Add(a.GetApolloServer())

	wsContainer.RecoverHandler(a.recoverFunc)
	wsContainer.Filter(a.process)

	return wsContainer, nil
}

// process 在接收和回复时统一处理请求
func (a *ApolloServer) process(req *restful.Request, rsp *restful.Response, chain *restful.FilterChain) {
	func() {
		if err := a.preprocess(req, rsp); err != nil {
			return
		}

		chain.ProcessFilter(req, rsp)
	}()

	a.postProcess(req, rsp)
}

// preprocess 请求预处理
func (a *ApolloServer) preprocess(req *restful.Request, rsp *restful.Response) error {
	// 设置开始时间
	req.SetAttribute("start-time", time.Now())

	// 处理请求ID
	reqId := req.HeaderParameter(types.HeaderRequestId)
	if reqId == "" {
		reqId = uuid.NewString()
	}

	// 打印请求
	apollolog.Info("receive request",
		zap.String("client-address", req.Request.RemoteAddr),
		zap.String("user-agent", req.HeaderParameter("User-Agent")),
		utils.ZapRequestID(reqId),
		zap.String("method", req.Request.Method),
		zap.String("url", req.Request.URL.String()),
	)

	// 限流
	if err := a.enterRateLimit(req, rsp); err != nil {
		return err
	}

	return nil
}

// postProcess 请求后处理：统计
func (a *ApolloServer) postProcess(req *restful.Request, rsp *restful.Response) {
	now := time.Now()

	// 接口调用统计
	path := req.Request.URL.Path
	if path != "/" {
		// 去掉最后一个"/"
		path = strings.TrimSuffix(path, "/")
	}
	method := req.Request.Method + ":" + path
	startTime := req.Attribute("start-time").(time.Time)
	code, ok := req.Attribute(utils.PolarisCode).(uint32)

	recordApiCall := true
	if !ok {
		code = uint32(rsp.StatusCode())
		recordApiCall = code != http.StatusNotFound && code != http.StatusMethodNotAllowed
	}

	diff := now.Sub(startTime)
	// 打印耗时超过1s的请求
	if diff > time.Second {
		apollolog.Info("handling time > 1s",
			zap.String("client-address", req.Request.RemoteAddr),
			zap.String("user-agent", req.HeaderParameter("User-Agent")),
			utils.ZapRequestID(req.HeaderParameter(types.HeaderRequestId)),
			zap.String("method", req.Request.Method),
			zap.String("url", req.Request.URL.String()),
			zap.Duration("handling-time", diff),
		)
	}

	if recordApiCall {
		statis.GetStatis().ReportCallMetrics(metrics.CallMetric{
			Type:     metrics.ServerCallMetric,
			API:      method,
			Protocol: "APOLLO-HTTP",
			Code:     int(code),
			Duration: diff,
		})
	}
}

// enterRateLimit 访问限制
func (a *ApolloServer) enterRateLimit(req *restful.Request, rsp *restful.Response) error {
	// 检查限流插件是否开启
	if ratelimit.GetRatelimit() == nil {
		return nil
	}

	rid := req.HeaderParameter(types.HeaderRequestId)
	// IP级限流
	// 先获取当前请求的address
	address := req.Request.RemoteAddr
	segments := strings.Split(address, ":")
	if len(segments) != 2 {
		return nil
	}
	if ok := ratelimit.GetRatelimit().Allow(ratelimit.IPRatelimit, segments[0]); !ok {
		apollolog.Error("ip ratelimit is not allow", zap.String("client", address),
			utils.ZapRequestID(rid))
		httputils.HTTPResponse(req, rsp, v1.NewResponse(apimodel.Code_IPRateLimit))
		return errors.New("ip ratelimit is not allow")
	}

	// 接口级限流
	apiName := fmt.Sprintf("%s:%s", req.Request.Method,
		strings.TrimSuffix(req.Request.URL.Path, "/"))
	if ok := ratelimit.GetRatelimit().Allow(ratelimit.APIRatelimit, apiName); !ok {
		apollolog.Error("api ratelimit is not allow", zap.String("client", address),
			utils.ZapRequestID(rid), zap.String("api", apiName))
		httputils.HTTPResponse(req, rsp, v1.NewResponse(apimodel.Code_APIRateLimit))
		return errors.New("api ratelimit is not allow")
	}

	return nil
}

func (a *ApolloServer) recoverFunc(i interface{}, w http.ResponseWriter) {
	apollolog.Errorf("panic %+v", i)
	obj := &apimodel.Response{}

	status := v1.CalcCode(obj)
	if code := obj.Code; code != v1.ExecuteSuccess {
		w.Header().Add(utils.PolarisCode, fmt.Sprintf("%d", code))
		w.Header().Add(utils.PolarisMessage, v1.Code2Info(code))
	}
	w.WriteHeader(status)

	data, _ := protojson.MarshalOptions{Indent: " ", EmitUnpopulated: true}.Marshal(obj)
	_, _ = w.Write(data)
}
