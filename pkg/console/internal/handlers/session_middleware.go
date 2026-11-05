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

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/common/log"
)

// RefreshJWTOnActivity extends a valid Console session after a successful
// request. Authentication remains the responsibility of the target handler;
// this middleware only centralizes sliding-session renewal.
func RefreshJWTOnActivity(config *bootstrap.Config) gin.HandlerFunc {
	loginPath := config.WebServer.AuthURL + "/user/login"
	return func(ctx *gin.Context) {
		if ctx.Request.Method == http.MethodOptions || ctx.Request.URL.Path == loginPath {
			ctx.Next()
			return
		}

		claims, err := parseJWTClaims(ctx, config)
		if err != nil || claims == nil {
			ctx.Next()
			return
		}

		writer := &sessionActivityWriter{
			ResponseWriter: ctx.Writer,
			renew: func() {
				if renewErr := refreshJWTWithRole(ctx, claims.UserID, claims.Token, claims.Role, config); renewErr != nil {
					log.Error("[Console][Session] refresh JWT after activity failed", zap.Error(renewErr))
				}
			},
		}
		ctx.Writer = writer
		ctx.Next()
		if !writer.Written() {
			writer.renewIfSuccessful(writer.Status())
		}
	}
}

type sessionActivityWriter struct {
	gin.ResponseWriter
	renew   func()
	checked bool
}

func (w *sessionActivityWriter) WriteHeader(code int) {
	w.renewIfSuccessful(code)
	w.ResponseWriter.WriteHeader(code)
}

func (w *sessionActivityWriter) WriteHeaderNow() {
	w.renewIfSuccessful(w.Status())
	w.ResponseWriter.WriteHeaderNow()
}

func (w *sessionActivityWriter) Write(data []byte) (int, error) {
	w.renewIfSuccessful(w.Status())
	return w.ResponseWriter.Write(data)
}

func (w *sessionActivityWriter) WriteString(data string) (int, error) {
	w.renewIfSuccessful(w.Status())
	return w.ResponseWriter.WriteString(data)
}

func (w *sessionActivityWriter) Flush() {
	w.renewIfSuccessful(w.Status())
	w.ResponseWriter.Flush()
}

func (w *sessionActivityWriter) renewIfSuccessful(status int) {
	if w.checked || status < http.StatusOK || status >= http.StatusMultipleChoices {
		return
	}
	w.checked = true
	if !responseHasJWTCookie(w.Header()) {
		w.renew()
	}
}

func responseHasJWTCookie(header http.Header) bool {
	for _, raw := range header.Values("Set-Cookie") {
		cookie, err := http.ParseSetCookie(raw)
		if err == nil && cookie.Name == "jwt" {
			return true
		}
	}
	return false
}
