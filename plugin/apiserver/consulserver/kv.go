package consulserver

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"

	"github.com/pole-io/pole-server/apis/pkg/types"
	compatconfig "github.com/pole-io/pole-server/pkg/compatibility/config"
)

type handler struct {
	reader      compatconfig.Discoverer
	environment string
	enabled     bool
}

// NewHandler 创建只读 KV profile。/v1/kv/group/name 与 dc 映射统一配置身份，label.* 只用于选择。
// index 是持久提交观察位置加一；允许无关提交导致提前返回，世代恢复导致 index 降低时客户端重新建基线。
func NewHandler(reader compatconfig.Discoverer, environment string, enabled bool) http.Handler {
	return &handler{reader: reader, environment: environment, enabled: enabled}
}

func (h *handler) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if !h.enabled || !strings.HasPrefix(request.URL.Path, "/v1/kv/") {
		http.NotFound(writer, request)
		return
	}
	if request.Method != http.MethodGet {
		writer.Header().Set("Allow", http.MethodGet)
		http.Error(writer, "configuration writes use the native management API", http.StatusMethodNotAllowed)
		return
	}
	key, group, name, valid := parseKey(strings.TrimPrefix(request.URL.Path, "/v1/kv/"))
	if !valid {
		http.Error(writer, "KV key must contain group/name", http.StatusBadRequest)
		return
	}
	query := request.URL.Query()
	for _, unsupported := range []string{"recurse", "keys", "separator", "ns", "partition", "stale", "cached"} {
		if query.Has(unsupported) {
			http.Error(writer, "unsupported KV query option", http.StatusBadRequest)
			return
		}
	}
	environment := query.Get("dc")
	if environment == "" {
		environment = h.environment
	}
	index := uint64(0)
	if query.Has("index") {
		value, err := strconv.ParseUint(query.Get("index"), 10, 64)
		if err != nil {
			http.Error(writer, "invalid blocking index", http.StatusBadRequest)
			return
		}
		index = value
	}
	wait := 60 * time.Second
	if query.Has("wait") {
		value, err := time.ParseDuration(query.Get("wait"))
		if err != nil || value < 0 || value > 10*time.Minute {
			http.Error(writer, "wait must be between 0s and 10m", http.StatusBadRequest)
			return
		}
		wait = value
	}
	if index == 0 {
		wait = 0
	}
	labels := make(map[string]string)
	for name, values := range query {
		if strings.HasPrefix(name, "label.") {
			if len(values) != 1 {
				http.Error(writer, "duplicate label", http.StatusBadRequest)
				return
			}
			labels[strings.TrimPrefix(name, "label.")] = values[0]
		}
	}
	input := &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: environment, Group: group, Name: name}, Labels: labels}
	ctx := requestContext(request)
	var result *compatconfig.Rendered
	var current uint64
	err := compatconfig.Wait(ctx, wait, func(ctx context.Context) (bool, error) {
		value, err := compatconfig.Read(ctx, h.reader, "consul", input)
		if err != nil {
			return false, err
		}
		cursor := value.Result.GetObservationCursor()
		if cursor == nil || cursor.Scope == "" || cursor.Epoch == "" || cursor.Position == math.MaxUint64 {
			return false, errors.New("configuration observation index unavailable")
		}
		result, current = value, cursor.Position+1
		return current != index, nil
	})
	if err != nil {
		status := http.StatusServiceUnavailable
		var failure *compatconfig.Error
		if errors.As(err, &failure) {
			status = failure.HTTPStatus()
		}
		http.Error(writer, "configuration read failed", status)
		return
	}
	writer.Header().Set("X-Consul-Index", strconv.FormatUint(current, 10))
	writer.Header().Set("X-Consul-Knownleader", "true")
	writer.Header().Set("X-Consul-Lastcontact", "0")
	writer.Header().Set("X-Pole-Config-Epoch", result.Result.ObservationCursor.Epoch)
	if result.Result.Presence == configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT {
		http.NotFound(writer, request)
		return
	}
	if query.Has("raw") && query.Get("raw") != "false" {
		writer.Header().Set("Content-Type", "text/plain")
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		_, _ = writer.Write(result.Content)
		return
	}
	writer.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(writer).Encode([]struct {
		Key         string
		Value       string
		CreateIndex uint64
		ModifyIndex uint64
		LockIndex   uint64
		Flags       uint64
	}{{Key: key, Value: base64.StdEncoding.EncodeToString(result.Content), ModifyIndex: current}})
}

func parseKey(value string) (key, group, name string, valid bool) {
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", "", false
	}
	return value, parts[0], parts[1], true
}

func requestContext(request *http.Request) context.Context {
	ctx := types.AppendRequestHeader(request.Context(), request.Header.Clone())
	token := request.Header.Get("X-Consul-Token")
	if token == "" {
		token = request.Header.Get("Authorization")
	}
	ctx = context.WithValue(ctx, types.ContextAuthTokenKey, token)
	return context.WithValue(ctx, types.ContextUserAgent, request.UserAgent())
}
