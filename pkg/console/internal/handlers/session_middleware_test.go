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
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
)

func TestRefreshJWTOnActivityExtendsSuccessfulSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := sessionMiddlewareTestConfig()
	conf.WebServer.JWT.Expired = 60
	oldCookie, err := newJWTCookieWithRole("user-1", "token-1", "sub", conf)
	require.NoError(t, err)
	conf.WebServer.JWT.Expired = 7200

	router := gin.New()
	router.Use(RefreshJWTOnActivity(conf))
	router.GET("/observability/v1/platform/overview", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"code": 200000})
	})
	request := httptest.NewRequest(http.MethodGet, "/observability/v1/platform/overview", nil)
	request.AddCookie(oldCookie)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	response := recorder.Result()
	require.Len(t, response.Cookies(), 1)
	refreshed := response.Cookies()[0]
	assert.Equal(t, "jwt", refreshed.Name)
	assert.Equal(t, conf.WebServer.JWT.Expired, refreshed.MaxAge)
	assert.NotEqual(t, oldCookie.Value, refreshed.Value)
}

func TestRefreshJWTOnActivityDoesNotExtendFailedSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := sessionMiddlewareTestConfig()
	cookie, err := newJWTCookieWithRole("user-1", "token-1", "sub", conf)
	require.NoError(t, err)

	router := gin.New()
	router.Use(RefreshJWTOnActivity(conf))
	router.GET("/observability/v1/platform/overview", func(ctx *gin.Context) {
		ctx.Status(http.StatusUnauthorized)
	})
	request := httptest.NewRequest(http.MethodGet, "/observability/v1/platform/overview", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusUnauthorized, recorder.Code)
	assert.Empty(t, recorder.Header().Values("Set-Cookie"))
}

func TestRefreshJWTOnActivityPreservesHandlerCookie(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := sessionMiddlewareTestConfig()
	oldCookie, err := newJWTCookieWithRole("user-1", "token-1", "sub", conf)
	require.NoError(t, err)

	router := gin.New()
	router.Use(RefreshJWTOnActivity(conf))
	router.GET("/auth/v1/user/session", func(ctx *gin.Context) {
		http.SetCookie(ctx.Writer, &http.Cookie{Name: "jwt", Value: "handler-cookie", Path: "/", MaxAge: 60})
		ctx.Status(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/auth/v1/user/session", nil)
	request.AddCookie(oldCookie)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, recorder.Result().Cookies(), 1)
	assert.Equal(t, "handler-cookie", recorder.Result().Cookies()[0].Value)
}

func TestRefreshJWTOnActivityDoesNotExtendLoginRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := sessionMiddlewareTestConfig()
	cookie, err := newJWTCookieWithRole("user-1", "token-1", "sub", conf)
	require.NoError(t, err)

	router := gin.New()
	router.Use(RefreshJWTOnActivity(conf))
	router.POST("/auth/v1/user/login", func(ctx *gin.Context) {
		ctx.Status(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodPost, "/auth/v1/user/login", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Empty(t, recorder.Header().Values("Set-Cookie"))
}

func TestRefreshJWTOnActivityExtendsStreamingSessionBeforeFlush(t *testing.T) {
	gin.SetMode(gin.TestMode)
	conf := sessionMiddlewareTestConfig()
	cookie, err := newJWTCookieWithRole("user-1", "token-1", "sub", conf)
	require.NoError(t, err)

	router := gin.New()
	router.Use(RefreshJWTOnActivity(conf))
	router.GET("/ai/agent/v1/stream", func(ctx *gin.Context) {
		ctx.Writer.Flush()
	})
	request := httptest.NewRequest(http.MethodGet, "/ai/agent/v1/stream", nil)
	request.AddCookie(cookie)
	recorder := httptest.NewRecorder()

	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Len(t, recorder.Result().Cookies(), 1)
	assert.Equal(t, "jwt", recorder.Result().Cookies()[0].Name)
}

func sessionMiddlewareTestConfig() *bootstrap.Config {
	conf := &bootstrap.Config{}
	conf.WebServer.AuthURL = "/auth/v1"
	conf.WebServer.JWT.Expired = 7200
	conf.WebServer.JWT.SecretKey = "session-middleware-secret"
	return conf
}
