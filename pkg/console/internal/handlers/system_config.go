package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/pole-io/pole-server/pkg/compatibility"
	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/agentworkbench"
	"github.com/pole-io/pole-server/pkg/console/internal/common/api"
	"github.com/pole-io/pole-server/pkg/console/internal/common/model"
	"github.com/pole-io/pole-server/pkg/console/internal/systemsettings"
	"github.com/pole-io/pole-server/pkg/systemconfig"
)

type SystemConfigurationView struct {
	Settings   []systemconfig.EffectiveSetting `json:"settings"`
	Components []SystemConfigurationComponent  `json:"components"`
}

type SystemConfigurationComponent struct {
	Name  systemconfig.Component `json:"name"`
	Count int                    `json:"count"`
}

func isSystemConfigurationAdminRole(role string) bool {
	return role == "main" || role == "admin"
}

func resolveSystemConfigurationSessionRole(claims *jwtClaims, adminGetter *AdminUserGetter) (string, bool) {
	if claims == nil {
		return "", false
	}
	if isSystemConfigurationAdminRole(claims.Role) {
		return claims.Role, true
	}
	// Cookies issued before the role claim was introduced are accepted only
	// when their signed user id matches the canonical Pole main account.
	if claims.Role == "" && adminGetter != nil {
		admin, err := adminGetter.GetAdminInfo()
		if err == nil && admin != nil && admin.GetId() != "" && admin.GetId() == claims.UserID {
			return "main", true
		}
	}
	return claims.Role, false
}

// RequireSystemConfigurationAdmin enforces the Console main-account/admin gate
// for both reads and every future draft, publish, secret and connection-test route.
func RequireSystemConfigurationAdmin(config *bootstrap.Config) gin.HandlerFunc {
	adminGetter := &AdminUserGetter{conf: config}
	return func(ctx *gin.Context) {
		_, _, ok := verifyAccessPermissionWithStatus(ctx, config, http.StatusUnauthorized)
		if !ok {
			ctx.Abort()
			return
		}
		claims, err := parseJWTClaims(ctx, config)
		_, isAdmin := resolveSystemConfigurationSessionRole(claims, adminGetter)
		isAdmin = err == nil && isAdmin
		if !isAdmin {
			writeSystemConfigurationError(ctx, http.StatusForbidden, errors.New("system configuration requires admin role"))
			ctx.Abort()
			return
		}
		ctx.Next()
	}
}

func DescribeSystemConfiguration(config *bootstrap.Config, manager *systemsettings.Manager) gin.HandlerFunc {
	consoleProvider, providerErr := systemsettings.NewSystemSettingsProvider(config)
	client := &http.Client{Timeout: bootstrap.DefaultAgentUpstreamTimeout}
	return func(ctx *gin.Context) {
		if providerErr != nil {
			writeSystemConfigurationError(ctx, http.StatusInternalServerError, providerErr)
			return
		}

		component, valid := parseSystemConfigurationComponent(ctx.Query("component"))
		if !valid {
			writeSystemConfigurationError(ctx, http.StatusBadRequest, fmt.Errorf("unsupported component %q", ctx.Query("component")))
			return
		}
		domain := strings.TrimSpace(ctx.Query("domain"))
		settings := make([]systemconfig.EffectiveSetting, 0)

		if component == "" || component == systemconfig.ComponentConsole {
			consoleSnapshot, err := consoleProvider.Effective(ctx.Request.Context(), systemconfig.Scope{
				Component: systemconfig.ComponentConsole,
				Domain:    domain,
			})
			if err != nil {
				writeSystemConfigurationError(ctx, http.StatusInternalServerError, err)
				return
			}
			settings = append(settings, consoleSnapshot.Settings...)
			manager.EnrichSettings(settings)
		}
		if component == "" || component == systemconfig.ComponentServer {
			serverSnapshot, err := loadServerSystemConfiguration(ctx, client, config)
			if err != nil {
				status := http.StatusBadGateway
				var upstreamError *systemConfigurationUpstreamError
				if errors.As(err, &upstreamError) && (upstreamError.Status == http.StatusUnauthorized || upstreamError.Status == http.StatusForbidden) {
					status = upstreamError.Status
				}
				writeSystemConfigurationError(ctx, status, err)
				return
			}
			for _, setting := range serverSnapshot.Settings {
				if domain == "" || setting.Domain == domain {
					settings = append(settings, setting)
				}
			}
		}

		sort.Slice(settings, func(i, j int) bool { return settings[i].Key < settings[j].Key })
		manager.EnrichManagedDesired(ctx.Request.Context(), settings)
		counts := map[systemconfig.Component]int{}
		for _, setting := range settings {
			counts[setting.Component]++
		}
		components := make([]SystemConfigurationComponent, 0, len(counts))
		for _, name := range []systemconfig.Component{systemconfig.ComponentServer, systemconfig.ComponentConsole} {
			if count := counts[name]; count > 0 {
				components = append(components, SystemConfigurationComponent{Name: name, Count: count})
			}
		}
		response := model.NewResponse(int32(api.ExecuteSuccess))
		response.Data = SystemConfigurationView{Settings: settings, Components: components}
		ctx.JSON(http.StatusOK, response)
	}
}

func GetSystemConfigurationDomain(config *bootstrap.Config, manager *systemsettings.Manager) gin.HandlerFunc {
	client := &http.Client{Timeout: bootstrap.DefaultAgentUpstreamTimeout}
	return func(ctx *gin.Context) {
		component, domain, ok := parseSystemConfigurationScope(ctx)
		if !ok {
			return
		}
		if component == systemsettings.AgentComponent && domain == systemsettings.AgentDomain {
			view, err := manager.Domain(ctx.Request.Context())
			if err != nil {
				writeSystemConfigurationError(ctx, http.StatusInternalServerError, err)
				return
			}
			writeSystemConfigurationSuccess(ctx, view)
			return
		}
		settings, err := loadScopedSystemSettings(ctx, client, config, systemconfig.Component(component), domain)
		if err != nil {
			writeSystemConfigurationError(ctx, http.StatusBadGateway, err)
			return
		}
		view, err := manager.ManagedDomain(ctx.Request.Context(), component, domain, settings)
		if err != nil {
			writeSystemConfigurationError(ctx, http.StatusInternalServerError, err)
			return
		}
		writeSystemConfigurationSuccess(ctx, view)
	}
}

func SaveSystemConfigurationDraft(config *bootstrap.Config, manager *systemsettings.Manager) gin.HandlerFunc {
	client := &http.Client{Timeout: bootstrap.DefaultAgentUpstreamTimeout}
	return func(ctx *gin.Context) {
		component, domain, ok := parseSystemConfigurationScope(ctx)
		if !ok {
			return
		}
		userID, token, ok := verifyAccessPermission(ctx, config)
		if !ok {
			return
		}
		if component == systemsettings.AgentComponent && domain == systemsettings.AgentDomain {
			var request systemsettings.SaveDraftRequest
			if err := ctx.ShouldBindJSON(&request); err != nil {
				writeSystemConfigurationError(ctx, http.StatusBadRequest, errors.New("Agent 配置草稿格式无效"))
				return
			}
			view, err := manager.SaveAndReconcile(ctx.Request.Context(), agentworkbench.Actor{
				UserID: userID, Token: token, RequestID: ctx.GetHeader("X-Request-Id"),
			}, request)
			if err != nil {
				// The desired revision was stored successfully, but its automatic
				// probe was rejected. Return the fresh domain so the UI receives
				// the new optimistic-lock revision and last-known-good status.
				if view != nil {
					writeSystemConfigurationSuccess(ctx, view)
					return
				}
				writeSystemConfigurationError(ctx, systemConfigurationMutationStatus(err), err)
				return
			}
			writeSystemConfigurationSuccess(ctx, view)
			return
		}
		var request systemsettings.SaveManagedDraftRequest
		if err := ctx.ShouldBindJSON(&request); err != nil {
			writeSystemConfigurationError(ctx, http.StatusBadRequest, errors.New("配置草稿格式无效"))
			return
		}
		if component == compatibility.Component && domain == compatibility.Domain {
			if err := validateServerManagedValues(ctx, client, config, component, domain, request.Values); err != nil {
				writeSystemConfigurationError(ctx, systemConfigurationMutationStatus(err), err)
				return
			}
		}
		settings, err := loadScopedSystemSettings(ctx, client, config, systemconfig.Component(component), domain)
		if err != nil {
			writeSystemConfigurationError(ctx, http.StatusBadGateway, err)
			return
		}
		view, err := manager.SaveManagedDraft(ctx.Request.Context(), userID, component, domain, settings, request)
		if err != nil {
			writeSystemConfigurationError(ctx, systemConfigurationMutationStatus(err), err)
			return
		}
		writeSystemConfigurationSuccess(ctx, view)
	}
}

func TestSystemConfigurationConnection(config *bootstrap.Config, manager *systemsettings.Manager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		if !requireAgentSystemConfigurationScope(ctx) {
			return
		}
		var request systemsettings.SaveDraftRequest
		if err := ctx.ShouldBindJSON(&request); err != nil {
			writeSystemConfigurationError(ctx, http.StatusBadRequest, errors.New("Agent 连接测试参数无效"))
			return
		}
		userID, token, ok := verifyAccessPermission(ctx, config)
		if !ok {
			return
		}
		result, err := manager.TestConnection(ctx.Request.Context(), agentworkbench.Actor{
			UserID: userID, Token: token, RequestID: ctx.GetHeader("X-Request-Id"),
		}, request)
		if err != nil {
			writeSystemConfigurationError(ctx, http.StatusBadGateway, err)
			return
		}
		writeSystemConfigurationSuccess(ctx, result)
	}
}

func PublishSystemConfiguration(config *bootstrap.Config, manager *systemsettings.Manager) gin.HandlerFunc {
	client := &http.Client{Timeout: bootstrap.DefaultAgentUpstreamTimeout}
	return func(ctx *gin.Context) {
		component, domain, ok := parseSystemConfigurationScope(ctx)
		if !ok {
			return
		}
		var request systemsettings.PublishRequest
		if err := ctx.ShouldBindJSON(&request); err != nil || request.DraftRevision <= 0 {
			writeSystemConfigurationError(ctx, http.StatusBadRequest, errors.New("草稿版本不能为空"))
			return
		}
		userID, token, ok := verifyAccessPermission(ctx, config)
		if !ok {
			return
		}
		if component == systemsettings.AgentComponent && domain == systemsettings.AgentDomain {
			view, err := manager.Publish(ctx.Request.Context(), agentworkbench.Actor{
				UserID: userID, Token: token, RequestID: ctx.GetHeader("X-Request-Id"),
			}, request)
			if err != nil {
				writeSystemConfigurationError(ctx, systemConfigurationMutationStatus(err), err)
				return
			}
			writeSystemConfigurationSuccess(ctx, view)
			return
		}
		if component == compatibility.Component && domain == compatibility.Domain {
			current, err := manager.ManagedDomain(ctx.Request.Context(), component, domain)
			if err != nil || current.Draft == nil || current.Draft.Revision != request.DraftRevision {
				writeSystemConfigurationError(ctx, http.StatusConflict, errors.New("草稿版本已变化，请刷新后重试"))
				return
			}
			if err = validateServerManagedValues(ctx, client, config, component, domain, current.Draft.Values); err != nil {
				writeSystemConfigurationError(ctx, systemConfigurationMutationStatus(err), err)
				return
			}
		}
		view, err := manager.PublishManaged(ctx.Request.Context(), userID, component, domain, request)
		if err != nil {
			writeSystemConfigurationError(ctx, systemConfigurationMutationStatus(err), err)
			return
		}
		writeSystemConfigurationSuccess(ctx, view)
	}
}

func validateServerManagedValues(ctx *gin.Context, client *http.Client, config *bootstrap.Config,
	component, domain string, values map[string]any) error {
	payload, err := json.Marshal(map[string]any{
		"component": component, "domain": domain, "values": values,
	})
	if err != nil {
		return err
	}
	target := fmt.Sprintf("http://%s/admin/v1/system/configuration/validate", config.PoleServer.Address)
	request, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodPost, target, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	for _, header := range []string{"Authorization", "X-Pole-User", "X-Request-Id"} {
		if value := ctx.Request.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("validate Pole Server system configuration: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= http.StatusOK && response.StatusCode < http.StatusMultipleChoices {
		return nil
	}
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	message := strings.TrimSpace(string(body))
	if message == "" {
		message = response.Status
	}
	return fmt.Errorf("动态配置校验失败: %s", message)
}

func ListSystemConfigurationReleases(manager *systemsettings.Manager) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		component, domain, ok := parseSystemConfigurationScope(ctx)
		if !ok {
			return
		}
		if component == systemsettings.AgentComponent && domain == systemsettings.AgentDomain {
			releases, err := manager.Releases(ctx.Request.Context())
			if err != nil {
				writeSystemConfigurationError(ctx, http.StatusInternalServerError, err)
				return
			}
			writeSystemConfigurationSuccess(ctx, releases)
			return
		}
		releases, err := manager.ManagedReleases(ctx.Request.Context(), component, domain)
		if err != nil {
			writeSystemConfigurationError(ctx, http.StatusInternalServerError, err)
			return
		}
		writeSystemConfigurationSuccess(ctx, releases)
	}
}

func requireAgentSystemConfigurationScope(ctx *gin.Context) bool {
	if ctx.Param("component") != systemsettings.AgentComponent || ctx.Param("domain") != systemsettings.AgentDomain {
		writeSystemConfigurationError(ctx, http.StatusNotFound, errors.New("该配置领域尚未开放动态编辑"))
		return false
	}
	return true
}

func parseSystemConfigurationScope(ctx *gin.Context) (string, string, bool) {
	component, valid := parseSystemConfigurationComponent(ctx.Param("component"))
	domain := strings.TrimSpace(ctx.Param("domain"))
	if !valid || component == "" || domain == "" {
		writeSystemConfigurationError(ctx, http.StatusBadRequest, errors.New("配置组件和领域无效"))
		return "", "", false
	}
	return string(component), domain, true
}

func loadScopedSystemSettings(ctx *gin.Context, client *http.Client, config *bootstrap.Config,
	component systemconfig.Component, domain string) ([]systemconfig.EffectiveSetting, error) {
	if component == systemconfig.ComponentConsole {
		provider, err := systemsettings.NewSystemSettingsProvider(config)
		if err != nil {
			return nil, err
		}
		snapshot, err := provider.Effective(ctx.Request.Context(), systemconfig.Scope{Component: component, Domain: domain})
		if err != nil {
			return nil, err
		}
		if len(snapshot.Settings) == 0 {
			return nil, errors.New("配置领域不存在")
		}
		return snapshot.Settings, nil
	}
	snapshot, err := loadServerSystemConfiguration(ctx, client, config)
	if err != nil {
		return nil, err
	}
	settings := make([]systemconfig.EffectiveSetting, 0)
	for _, setting := range snapshot.Settings {
		if setting.Domain == domain {
			settings = append(settings, setting)
		}
	}
	if len(settings) == 0 {
		return nil, errors.New("配置领域不存在")
	}
	return settings, nil
}

func systemConfigurationMutationStatus(err error) int {
	message := err.Error()
	if strings.Contains(message, "连接校验失败") || strings.Contains(message, "Gateway") ||
		strings.Contains(message, "MCP") {
		return http.StatusBadGateway
	}
	if strings.Contains(message, "版本") || strings.Contains(message, "conflict") {
		return http.StatusConflict
	}
	if strings.Contains(message, "不能为空") || strings.Contains(message, "无效") ||
		strings.Contains(message, "必须") || strings.Contains(message, "不支持") ||
		strings.Contains(message, "未配置") || strings.Contains(message, "动态配置校验失败") {
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}

func writeSystemConfigurationSuccess(ctx *gin.Context, data any) {
	response := model.NewResponse(int32(api.ExecuteSuccess))
	response.Data = data
	ctx.JSON(http.StatusOK, response)
}

type systemConfigurationUpstreamError struct {
	Status int
}

func (e *systemConfigurationUpstreamError) Error() string {
	return fmt.Sprintf("Pole Server effective configuration returned HTTP %d", e.Status)
}

func loadServerSystemConfiguration(ctx *gin.Context, client *http.Client,
	config *bootstrap.Config) (*systemconfig.EffectiveSnapshot, error) {
	target := fmt.Sprintf("http://%s/admin/v1/system/configuration", config.PoleServer.Address)
	request, err := http.NewRequestWithContext(ctx.Request.Context(), http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	for _, header := range []string{"Authorization", "X-Pole-User", "X-Request-Id"} {
		if value := ctx.Request.Header.Get(header); value != "" {
			request.Header.Set(header, value)
		}
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("read Pole Server effective configuration: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, &systemConfigurationUpstreamError{Status: response.StatusCode}
	}
	snapshot := &systemconfig.EffectiveSnapshot{}
	if err := json.NewDecoder(response.Body).Decode(snapshot); err != nil {
		return nil, fmt.Errorf("decode Pole Server effective configuration: %w", err)
	}
	return snapshot, nil
}

func parseSystemConfigurationComponent(value string) (systemconfig.Component, bool) {
	switch systemconfig.Component(strings.TrimSpace(value)) {
	case "":
		return "", true
	case systemconfig.ComponentServer:
		return systemconfig.ComponentServer, true
	case systemconfig.ComponentConsole:
		return systemconfig.ComponentConsole, true
	default:
		return "", false
	}
}

func writeSystemConfigurationError(ctx *gin.Context, status int, err error) {
	response := model.NewResponse(int32(api.ExecuteException))
	response.Data = err.Error()
	ctx.JSON(status, response)
}
