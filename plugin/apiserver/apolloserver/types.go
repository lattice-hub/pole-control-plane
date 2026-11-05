package apolloserver

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"

	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
)

func newErrorResponse(req *restful.Request, err error) *ErrorResponse {
	var protocolError *compatconfig.Error
	if errors.As(err, &protocolError) {
		status := protocolError.HTTPStatus()
		return &ErrorResponse{Timestamp: time.Now().String(), Status: status, Error: http.StatusText(status), Message: err.Error(), Path: req.Request.URL.Path}
	}
	if errors.Is(err, Err_NotFoundConfig) {
		return &ErrorResponse{
			Timestamp: time.Now().String(),
			Status:    http.StatusNotFound,
			Error:     http.StatusText(http.StatusNotFound),
			Message:   err.Error(),
			Path:      req.Request.URL.Path,
		}
	}

	return &ErrorResponse{
		Timestamp: time.Now().String(),
		Status:    http.StatusInternalServerError,
		Error:     http.StatusText(http.StatusInternalServerError),
		Message:   err.Error(),
		Path:      req.Request.URL.Path,
	}
}

type ErrorResponse struct {
	Timestamp string `json:"timestamp"`
	Status    int    `json:"status"`
	Error     string `json:"error"`
	Message   string `json:"message"`
	Path      string `json:"path"`
}

func (e *ErrorResponse) GetCode() uint32 {
	return uint32(e.Status)
}

func (e *ErrorResponse) GetInfo() string {
	return e.Message
}

type GetConfigFileRequest struct {
	AppId      string
	Cluster    string
	DataCenter string
	Filename   string
	ClientIP   string
	Version    string
}

type GetConfigFileResponse struct {
	AppId          string            `json:"appId"`
	Cluster        string            `json:"cluster"`
	NamespaceName  string            `json:"namespaceName"`
	Content        []byte            `json:"-"`
	Configurations map[string]string `json:"configurations"`
	ReleaseKey     string            `json:"releaseKey"`
}

type WatchConfigFileRequest struct {
	AppId         string
	Cluster       string
	DataCenter    string
	ClientIP      string
	Notifications []*ApolloConfigNotification
}

type ApolloConfigNotification struct {
	NamespaceName  string                      `json:"namespaceName"`
	NotificationId int64                       `json:"notificationId"`
	Messages       *ApolloNotificationMessages `json:"messages"`
}

type ApolloNotificationMessages struct {
	Details map[string]int64 `json:"details"`
}

type WatchConfigFileResponse struct {
	Notifications []*ApolloConfigNotification `json:"notifications"`
}

// ServerNode represents a service instance
type ServerNode struct {
	AppName     string `json:"appName"`
	InstanceId  string `json:"instanceId"`
	HomepageUrl string `json:"homepageUrl"`
}

func PropertiesToString(props map[string]string) string {
	if len(props) == 0 {
		return "{}"
	}

	pairs := make([]string, 0, len(props))
	for k, v := range props {
		pairs = append(pairs, fmt.Sprintf("%s=%s", k, v))
	}
	return fmt.Sprintf("{%s}", strings.Join(pairs, ", "))
}
