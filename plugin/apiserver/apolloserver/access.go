package apolloserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/emicklei/go-restful/v3"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"

	httputils "github.com/pole-io/pole-server/pkg/common/utils/http"
)

const (
	Tpl_NotFoundConfig = "Could not load configurations with appId: %s, clusterName: %s, namespace: %s"
)

// GetApolloServer apollo web server
func (a *ApolloServer) GetApolloServer() *restful.WebService {
	ws := new(restful.WebService)
	ws.Path("/").Consumes(restful.MIME_JSON, restful.MIME_OCTET, restful.MIME_XML).Produces(restful.MIME_JSON,
		restful.MIME_XML)
	a.addClientAccess(ws)
	a.addMetaAccess(ws)
	return ws
}

func (a *ApolloServer) addClientAccess(ws *restful.WebService) {
	ws.Route(ws.GET("/configs/{appId}/{cluster}/{namespace}").To(a.fetchConfig).
		Param(ws.PathParameter("appId", "appId").DataType("string")).
		Param(ws.PathParameter("cluster", "cluster").DataType("string")).
		Param(ws.PathParameter("namespace", "namespace").DataType("string")))
	ws.Route(ws.GET("/configfiles/{appId}/{cluster}/{namespace}").To(a.queryProperties).
		Param(ws.PathParameter("appId", "appId").DataType("string")).
		Param(ws.PathParameter("cluster", "cluster").DataType("string")).
		Param(ws.PathParameter("namespace", "namespace").DataType("string")))
	ws.Route(ws.GET("/configfiles/json/{appId}/{cluster}/{namespace}").To(a.queryJson).
		Param(ws.PathParameter("appId", "appId").DataType("string")).
		Param(ws.PathParameter("cluster", "cluster").DataType("string")).
		Param(ws.PathParameter("namespace", "namespace").DataType("string")))
	ws.Route(ws.GET("/notifications/v2").To(a.watchConfig))
}

// fetchConfig 处理 apollo 客户端获取配置文件
func (a *ApolloServer) fetchConfig(req *restful.Request, rsp *restful.Response) {
	appId := req.PathParameter("appId")
	cluster := req.PathParameter("cluster")
	filename := req.PathParameter("namespace")

	ret, err := a.GetConfigFile(httputils.ParseHeaderContext(req, rsp), &GetConfigFileRequest{
		AppId:      appId,
		Cluster:    cluster,
		DataCenter: req.QueryParameter("dataCenter"),
		Filename:   filename,
		ClientIP:   req.QueryParameter("ip"),
		Version:    req.QueryParameter("releaseKey"),
	})

	if err != nil {
		errRsp := newErrorResponse(req, err)
		if errRsp.Status == http.StatusNotFound {
			errRsp.Message = fmt.Sprintf(Tpl_NotFoundConfig, appId, cluster, filename)
		}
		_ = rsp.WriteHeaderAndJson(errRsp.Status, errRsp, restful.MIME_JSON)
		return
	}

	// 相同快照返回304。
	if ret == nil {
		rsp.WriteHeader(http.StatusNotModified)
		return
	}

	// 返回正常的数据
	_ = rsp.WriteHeaderAndJson(http.StatusOK, ret, restful.MIME_JSON)
}

// queryProperties 处理 apollo 客户端获取配置文件
func (a *ApolloServer) queryProperties(req *restful.Request, rsp *restful.Response) {
	result, err := a.readConfig(httputils.ParseHeaderContext(req, rsp), &GetConfigFileRequest{AppId: req.PathParameter("appId"), Cluster: req.PathParameter("cluster"), DataCenter: req.QueryParameter("dataCenter"), Filename: req.PathParameter("namespace"), ClientIP: req.QueryParameter("ip")})
	if err == nil && result.Result.Presence != configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT {
		err = Err_NotFoundConfig
	}
	if err != nil {
		failure := newErrorResponse(req, err)
		_ = rsp.WriteHeaderAndJson(failure.Status, failure, restful.MIME_JSON)
		return
	}
	httputils.HTTPRawResponse(rsp, http.StatusOK, map[string]string{restful.HEADER_ContentType: "text/plain;charset=UTF-8"}, result.Content)
}

// queryJson 处理 apollo 客户端获取配置文件
func (a *ApolloServer) queryJson(req *restful.Request, rsp *restful.Response) {
	appId := req.PathParameter("appId")
	cluster := req.PathParameter("cluster")
	filename := req.PathParameter("namespace")

	ret, err := a.GetConfigFile(httputils.ParseHeaderContext(req, rsp), &GetConfigFileRequest{
		AppId:      appId,
		Cluster:    cluster,
		DataCenter: req.QueryParameter("dataCenter"),
		Filename:   filename,
		ClientIP:   req.QueryParameter("ip"),
		Version:    req.QueryParameter("releaseKey"),
	})

	if err != nil {
		errRsp := newErrorResponse(req, err)
		if errRsp.Status == http.StatusNotFound {
			errRsp.Message = fmt.Sprintf(Tpl_NotFoundConfig, appId, cluster, filename)
		}
		_ = rsp.WriteHeaderAndJson(errRsp.Status, errRsp, restful.MIME_JSON)
		return
	}

	// 如果没有数据，代表数据没有出现变化，返回304
	if ret == nil {
		rsp.WriteHeader(http.StatusNotModified)
		return
	}

	d, _ := json.Marshal(ret.Configurations)
	// 返回正常的数据
	httputils.HTTPRawResponse(rsp, http.StatusOK, map[string]string{
		restful.HEADER_ContentType: "application/json;charset=UTF-8",
	}, d)
}

// watchConfig 处理 apollo 客户端配置变更监听
func (a *ApolloServer) watchConfig(req *restful.Request, rsp *restful.Response) {
	appId := req.QueryParameter("appId")
	cluster := req.QueryParameter("cluster")
	dataCenter := req.QueryParameter("dataCenter")
	clientIp := req.QueryParameter("ip")
	notifications := []*ApolloConfigNotification{}
	if err := json.Unmarshal([]byte(req.QueryParameter("notifications")), &notifications); err != nil {
		errRsp := newErrorResponse(req, err)
		_ = rsp.WriteHeaderAndJson(errRsp.Status, errRsp, restful.MIME_JSON)
		return
	}

	if len(notifications) == 0 {
		// TODO 如果没有变更通知，直接返回空
		return
	}

	ret, err := a.WatchConfigFile(httputils.ParseHeaderContext(req, rsp), &WatchConfigFileRequest{
		AppId:         appId,
		Cluster:       cluster,
		DataCenter:    dataCenter,
		ClientIP:      clientIp,
		Notifications: notifications,
	})
	if err != nil {
		errRsp := newErrorResponse(req, err)
		_ = rsp.WriteHeaderAndJson(errRsp.Status, errRsp, restful.MIME_JSON)
		return
	}

	// Apollo 长轮询直接返回通知数组，超时无变化返回304。
	if len(ret.Notifications) == 0 {
		rsp.WriteHeader(http.StatusNotModified)
		return
	}
	_ = rsp.WriteHeaderAndJson(http.StatusOK, ret.Notifications, restful.MIME_JSON)
}

func (a *ApolloServer) addMetaAccess(ws *restful.WebService) {
	ws.Route(ws.GET("/services/config").To(a.fetchMeta))
}

func (a *ApolloServer) fetchMeta(req *restful.Request, rsp *restful.Response) {
	appId := req.QueryParameter("appId")
	clientIp := req.QueryParameter("ip")

	_ = appId
	_ = clientIp

	nodes, err := a.GetConfigServers(httputils.ParseHeaderContext(req, rsp), appId, clientIp)
	if err != nil {
		errRsp := newErrorResponse(req, err)
		_ = rsp.WriteHeaderAndJson(errRsp.Status, errRsp, restful.MIME_JSON)
		return
	}

	_ = rsp.WriteHeaderAndJson(http.StatusOK, nodes, restful.MIME_JSON)
}
