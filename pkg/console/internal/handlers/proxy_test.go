/**
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */

package handlers

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	globalotel "go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/specification/source/go/api/v1/security"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReverseProxyNoAuthPropagatesTraceContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousProvider := globalotel.GetTracerProvider()
	previousPropagator := globalotel.GetTextMapPropagator()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	globalotel.SetTracerProvider(provider)
	globalotel.SetTextMapPropagator(propagation.TraceContext{})
	t.Cleanup(func() {
		globalotel.SetTracerProvider(previousProvider)
		globalotel.SetTextMapPropagator(previousPropagator)
	})

	traceparent := make(chan string, 1)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		traceparent <- req.Header.Get("traceparent")
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	conf := &bootstrap.Config{}
	conf.PoleServer.Address = backendAddress(backend.URL)
	router := gin.New()
	router.GET("/admin/v1/server/functions", ReverseProxyNoAuthForServer(&conf.PoleServer, conf))
	console := httptest.NewServer(otelhttp.NewHandler(router, "console"))
	defer console.Close()

	ctx, parent := provider.Tracer("test").Start(context.Background(), "console request")
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, console.URL+"/admin/v1/server/functions", nil)
	require.NoError(t, err)
	globalotel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
	rsp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer rsp.Body.Close()
	parent.End()

	assert.Equal(t, http.StatusOK, rsp.StatusCode)
	assert.NotEmpty(t, <-traceparent)
	spans := recorder.Ended()
	require.Len(t, spans, 3)
	var proxySpan sdktrace.ReadOnlySpan
	var consoleSpan sdktrace.ReadOnlySpan
	for _, span := range spans {
		switch span.Name() {
		case "proxy GET:/admin/v1/server/functions":
			proxySpan = span
		case "console":
			consoleSpan = span
		}
	}
	require.NotNil(t, proxySpan)
	require.NotNil(t, consoleSpan)
	assert.Equal(t, consoleSpan.SpanContext().SpanID(), proxySpan.Parent().SpanID())
	assert.Equal(t, parent.SpanContext().TraceID(), proxySpan.SpanContext().TraceID())
}

func TestRefreshJWTAndParse(t *testing.T) {
	conf := &bootstrap.Config{}
	conf.WebServer.JWT.Expired = 2
	conf.WebServer.JWT.SecretKey = "polarismesh@2021"
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	userID, token := "user1", "token1"
	err := refreshJWT(c, userID, token, conf)
	assert.NoError(t, err, "refresh jwt should not fail")
	time.Sleep(time.Second)

	c.Request = &http.Request{Header: http.Header{}}
	c.Request.Header.Set("Cookie", c.Writer.Header().Get("Set-Cookie"))
	c.Request.Header.Set("x-pole-user", userID)
	targetUserID, targetToken, err := parseJWTThenSetToken(c, conf)
	assert.NoError(t, err, "parse jwt should not fail")
	if targetUserID != userID {
		t.Errorf("user id should be same to target")
	}
	if targetToken != token {
		t.Errorf("token should be same to target")
	}
	time.Sleep(2 * time.Second)
	_, _, err = parseJWTThenSetToken(c, conf)
	assert.Error(t, err, "token should be expire")
}

func TestReverseProxyForLoginSetsJWTFromStandardData(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/v1/mainuser/exist" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		require.Equal(t, "/auth/v1/user/login", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200000,"data":{"user_id":"user1","name":"admin","role":"main","token":"token1"}}`))
	}))
	defer backend.Close()

	conf := &bootstrap.Config{}
	conf.WebServer.JWT.Expired = 1800
	conf.WebServer.JWT.SecretKey = "polarismesh@2021"
	conf.WebServer.MainUser = "pole"
	conf.PoleServer.Address = backendAddress(backend.URL)
	NewAdminGetter(conf)

	router := gin.New()
	router.POST("/auth/v1/user/login", ReverseProxyForLogin(&conf.PoleServer, conf))
	console := httptest.NewServer(router)
	defer console.Close()

	resp, err := http.Post(console.URL+"/auth/v1/user/login", "application/json", strings.NewReader("{}"))
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, resp.Cookies())
	assert.Equal(t, "jwt", resp.Cookies()[0].Name)
	assert.True(t, resp.Cookies()[0].HttpOnly)
	request := &http.Request{Header: http.Header{}}
	request.AddCookie(resp.Cookies()[0])
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = request
	claims, err := parseJWTClaims(ctx, conf)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "main", claims.Role)
}

func TestDescribeConsoleSessionUsesSignedRole(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := &bootstrap.Config{}
	conf.WebServer.JWT.Expired = 1800
	conf.WebServer.JWT.SecretKey = "session-secret"

	for _, testCase := range []struct {
		name       string
		role       string
		wantStatus int
		wantAdmin  bool
	}{
		{name: "admin", role: "admin", wantStatus: http.StatusOK, wantAdmin: true},
		{name: "main", role: "main", wantStatus: http.StatusOK, wantAdmin: true},
		{name: "non admin", role: "sub", wantStatus: http.StatusOK, wantAdmin: false},
		{name: "missing session", role: "", wantStatus: http.StatusUnauthorized, wantAdmin: false},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest(http.MethodGet, "/auth/v1/user/session", nil)
			if testCase.role != "" {
				cookieRecorder := httptest.NewRecorder()
				cookieCtx, _ := gin.CreateTestContext(cookieRecorder)
				require.NoError(t, refreshJWTWithRole(cookieCtx, "user-1", "token-1", testCase.role, conf))
				ctx.Request.Header.Set("Cookie", cookieRecorder.Header().Get("Set-Cookie"))
			}

			DescribeConsoleSession(conf)(ctx)
			require.Equal(t, testCase.wantStatus, recorder.Code)
			if testCase.wantStatus == http.StatusOK {
				assert.Contains(t, recorder.Body.String(), `"role":"`+testCase.role+`"`)
				assert.Contains(t, recorder.Body.String(), fmt.Sprintf(`"admin":%t`, testCase.wantAdmin))
			}
		})
	}
}

func TestResolveSystemConfigurationSessionRoleUpgradesLegacyMainCookie(t *testing.T) {
	getter := &AdminUserGetter{user: &security.User{Id: "user-1"}}
	role, admin := resolveSystemConfigurationSessionRole(&jwtClaims{
		UserID: "user-1",
		Token:  "token-1",
	}, getter)
	assert.Equal(t, "main", role)
	assert.True(t, admin)

	role, admin = resolveSystemConfigurationSessionRole(&jwtClaims{
		UserID: "ordinary-user",
		Token:  "token-2",
	}, getter)
	assert.Empty(t, role)
	assert.False(t, admin)
}

func TestReverseProxyForServerRefreshesJWTAfterBackendSuccess(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/naming/v1/services", r.URL.Path)
		assert.Equal(t, "token1", r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200000,"data":[]}`))
	}))
	defer backend.Close()

	conf := testProxyConfig(backend.URL)
	router := gin.New()
	router.GET("/naming/v1/services", ReverseProxyForServer(&conf.PoleServer, conf))
	console := httptest.NewServer(router)
	defer console.Close()

	req, err := http.NewRequest(http.MethodGet, console.URL+"/naming/v1/services", nil)
	require.NoError(t, err)
	req.AddCookie(testJWTCookieWithRole(t, "user1", "token1", "main", conf))
	req.Header.Set("x-pole-user", "user1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.NotEmpty(t, resp.Cookies())
	assert.Equal(t, "jwt", resp.Cookies()[0].Name)
	assert.NotEmpty(t, resp.Cookies()[0].Value)

	refreshedRequest := &http.Request{Header: http.Header{}}
	refreshedRequest.AddCookie(resp.Cookies()[0])
	refreshedContext, _ := gin.CreateTestContext(httptest.NewRecorder())
	refreshedContext.Request = refreshedRequest
	claims, err := parseJWTClaims(refreshedContext, conf)
	require.NoError(t, err)
	require.NotNil(t, claims)
	assert.Equal(t, "main", claims.Role)
}

func TestReverseProxyForServerPreservesJWTAfterAuthorizationDenied(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401001,"info":"access is not approved"}`))
	}))
	defer backend.Close()

	conf := testProxyConfig(backend.URL)
	router := gin.New()
	router.GET("/naming/v1/services", ReverseProxyForServer(&conf.PoleServer, conf))
	console := httptest.NewServer(router)
	defer console.Close()

	req, err := http.NewRequest(http.MethodGet, console.URL+"/naming/v1/services", nil)
	require.NoError(t, err)
	req.AddCookie(testJWTCookie(t, "user1", "stale-token", conf))
	req.Header.Set("x-pole-user", "user1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	assert.Empty(t, resp.Cookies())
}

func TestReverseProxyForServerClearsJWTAfterAuthenticationInvalidated(t *testing.T) {
	gin.SetMode(gin.TestMode)

	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401004,"info":"token not exist"}`))
	}))
	defer backend.Close()

	conf := testProxyConfig(backend.URL)
	router := gin.New()
	router.GET("/naming/v1/services", ReverseProxyForServer(&conf.PoleServer, conf))
	console := httptest.NewServer(router)
	defer console.Close()

	req, err := http.NewRequest(http.MethodGet, console.URL+"/naming/v1/services", nil)
	require.NoError(t, err)
	req.AddCookie(testJWTCookie(t, "user1", "stale-token", conf))
	req.Header.Set("x-pole-user", "user1")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	require.NotEmpty(t, resp.Cookies())
	assert.Equal(t, "jwt", resp.Cookies()[0].Name)
	assert.Empty(t, resp.Cookies()[0].Value)
	assert.LessOrEqual(t, resp.Cookies()[0].MaxAge, 0)
}

func testProxyConfig(backendURL string) *bootstrap.Config {
	conf := &bootstrap.Config{}
	conf.WebServer.JWT.Expired = 1800
	conf.WebServer.JWT.SecretKey = "polarismesh@2021"
	conf.PoleServer.Address = backendAddress(backendURL)
	return conf
}

func testJWTCookie(t *testing.T, userID string, token string, conf *bootstrap.Config) *http.Cookie {
	t.Helper()
	cookie, err := newJWTCookie(userID, token, conf)
	require.NoError(t, err)
	require.NotNil(t, cookie)
	return cookie
}

func testJWTCookieWithRole(t *testing.T, userID string, token string, role string, conf *bootstrap.Config) *http.Cookie {
	t.Helper()
	cookie, err := newJWTCookieWithRole(userID, token, role, conf)
	require.NoError(t, err)
	require.NotNil(t, cookie)
	return cookie
}

func backendAddress(rawURL string) string {
	_, port, err := net.SplitHostPort(strings.TrimPrefix(rawURL, "http://"))
	if err == nil {
		return "127.0.0.1:" + port
	}
	return strings.TrimPrefix(rawURL, "http://")
}
