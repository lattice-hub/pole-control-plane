package apolloserver

import (
	"bytes"
	"context"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/magiconair/properties"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	"github.com/pole-io/specification/source/go/api/v1/service_manage"

	"github.com/pole-io/pole-server/apis/pkg/types"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/common/utils"
	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
)

// readConfig 保留 Apollo cluster 定位顺序，仅 ABSENT 允许查询下一个名称；拒绝或失败立即返回。
func (a *ApolloServer) readConfig(ctx context.Context, req *GetConfigFileRequest) (*compatconfig.Rendered, error) {
	environments := []string{req.Cluster, req.DataCenter, "default"}
	filenames := []string{req.Filename}
	if path.Ext(req.Filename) == "" {
		filenames = append(filenames, req.Filename+".properties")
	}
	var last *compatconfig.Rendered
	seen := map[string]bool{}
	for _, environment := range environments {
		if environment == "" || seen[environment] {
			continue
		}
		seen[environment] = true
		for _, filename := range filenames {
			// ip 是 Apollo 请求提供的选择标签，不能充当认证身份。
			labels := map[string]string{types.ClientLabel_IP: req.ClientIP}
			if req.ClientIP == "" {
				labels[types.ClientLabel_IP] = utils.ParseClientIP(ctx)
			}
			result, err := compatconfig.Read(ctx, a.configSvr, "apollo", &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: environment, Group: req.AppId, Name: filename}, Labels: labels})
			if err != nil {
				return nil, err
			}
			last = result
			if result.Result.Presence == configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT {
				return result, nil
			}
		}
	}
	if last != nil {
		return last, nil
	}
	return nil, Err_NotFoundConfig
}

func (a *ApolloServer) GetConfigFile(ctx context.Context, req *GetConfigFileRequest) (*GetConfigFileResponse, error) {
	rendered, err := a.readConfig(ctx, req)
	if err != nil {
		return nil, err
	}
	if rendered.Result.Presence == configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT {
		return nil, Err_NotFoundConfig
	}
	if !utf8.Valid(rendered.Content) {
		return nil, &compatconfig.Error{Code: apimodel.Code_ConfigRenderFailed}
	}
	revision := rendered.Result.SnapshotRevision
	if revision == req.Version {
		return nil, nil
	}
	configurations := map[string]string{"content": string(rendered.Content)}
	format := strings.ToLower(rendered.Result.Snapshot.Format)
	if format == "properties" || path.Ext(req.Filename) == ".properties" || (format == "" && path.Ext(req.Filename) == "") {
		// 禁止properties库解释${...}；解析副本统一CRLF以修正库的续行行为，Content不改写。
		parsed, err := (&properties.Loader{Encoding: properties.UTF8, DisableExpansion: true}).LoadBytes(bytes.ReplaceAll(rendered.Content, []byte("\r\n"), []byte("\n")))
		if err != nil {
			return nil, &compatconfig.Error{Code: apimodel.Code_ConfigRenderFailed}
		}
		configurations = parsed.Map()
	}
	return &GetConfigFileResponse{AppId: req.AppId, Cluster: req.Cluster, NamespaceName: req.Filename, ReleaseKey: revision, Configurations: configurations, Content: rendered.Content}, nil
}

// WatchConfigFile 最多等待60秒，每次重读鉴权；通知版本来自同一持久快照的游标。
func (a *ApolloServer) WatchConfigFile(ctx context.Context, req *WatchConfigFileRequest) (*WatchConfigFileResponse, error) {
	if len(req.Notifications) > 128 {
		return nil, fmt.Errorf("at most 128 notifications are supported")
	}
	result := &WatchConfigFileResponse{}
	timeout := a.watchTimeOut
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	err := compatconfig.Wait(ctx, timeout, func(ctx context.Context) (bool, error) {
		result.Notifications = nil
		for _, notification := range req.Notifications {
			if notification == nil {
				return false, fmt.Errorf("invalid notification")
			}
			rendered, err := a.readConfig(ctx, &GetConfigFileRequest{AppId: req.AppId, Cluster: req.Cluster, DataCenter: req.DataCenter, Filename: notification.NamespaceName, ClientIP: req.ClientIP})
			if err != nil {
				return false, err
			}
			position := rendered.Result.GetObservationCursor().GetPosition()
			if position > uint64(^uint64(0)>>1) {
				return false, fmt.Errorf("notification cursor cannot be represented")
			}
			version := int64(position)
			if version != notification.NotificationId {
				result.Notifications = append(result.Notifications, &ApolloConfigNotification{NamespaceName: notification.NamespaceName, NotificationId: version})
			}
		}
		return len(result.Notifications) > 0, nil
	})
	return result, err
}

// GetConfigServers 获取配置中心的接入点，二次寻址发现
func (a *ApolloServer) GetConfigServers(ctx context.Context, appId, clientIp string) ([]*ServerNode, error) {
	if a.metaSvrs == nil {
		return []*ServerNode{
			{
				AppName:     a.GetProtocol(),
				InstanceId:  fmt.Sprintf("http://%s:%d/", utils.LocalHost, a.listenPort),
				HomepageUrl: fmt.Sprintf("http://%s:%d/", utils.LocalHost, a.listenPort),
			},
		}, nil
	}

	rsp := a.discoverSvr.ServiceInstancesCache(ctx, &service_manage.DiscoverFilter{
		OnlyHealthyInstance: true,
	}, &service_manage.Service{
		Namespace: a.metaSvrs.Namespace,
		Name:      a.metaSvrs.Name,
	})

	if !api.IsSuccess(rsp) {
		return nil, nil
	}

	nodes := make([]*ServerNode, 0, len(rsp.GetInstances()))
	for _, ins := range rsp.GetInstances() {
		nodes = append(nodes, &ServerNode{
			AppName:     ins.GetService(),
			InstanceId:  ins.GetId(),
			HomepageUrl: fmt.Sprintf("http://%s:%d/", ins.GetHost(), ins.GetPort()),
		})
	}
	return nodes, nil
}
