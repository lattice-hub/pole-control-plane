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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httputil"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"

	bootstrap "github.com/pole-io/pole-server/pkg/console/config"
	"github.com/pole-io/pole-server/pkg/console/internal/common/log"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
	"github.com/pole-io/specification/source/go/api/v1/security"
)

var proxyTransport = otelhttp.NewTransport(http.DefaultTransport,
	otelhttp.WithSpanNameFormatter(func(_ string, request *http.Request) string {
		return "proxy " + request.Method + ":" + request.URL.Path
	}),
)

func NewAdminGetter(conf *bootstrap.Config) {
	_adminGetter.conf = conf
}

var _adminGetter = &AdminUserGetter{}

type AdminUserGetter struct {
	conf *bootstrap.Config
	lock sync.RWMutex
	user *security.User
}

func (a *AdminUserGetter) GetAdminInfo() (*security.User, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	if a.user != nil {
		return a.user, nil
	}

	resp, err := http.Get(fmt.Sprintf("http://%s/admin/v1/mainuser/exist", a.conf.PoleServer.Address))
	if err != nil || resp.StatusCode != http.StatusOK {
		user := &security.User{
			Name: a.conf.WebServer.MainUser,
		}
		if resp != nil {
			// 降级回旧的数据信息
			if resp.StatusCode == http.StatusNotFound {
				a.user = user
			} else {
				log.Error("[Proxy][Login] get admin info fail", zap.Error(err))
			}
		}
		return user, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("[Proxy][Login] get admin info fail", zap.Error(err))
		return nil, err
	}
	rsp := &apimodel.Response{}
	unmarshaler := protojson.UnmarshalOptions{DiscardUnknown: true}
	if err = unmarshaler.Unmarshal(body, rsp); err != nil {
		log.Error("[Proxy][Login] get admin info fail", zap.Error(err))
		return nil, err
	}
	user := &security.User{}
	if rsp.GetData() == nil {
		return nil, errors.New("admin user data is empty")
	}
	if err = rsp.GetData().UnmarshalTo(user); err != nil {
		log.Error("[Proxy][Login] get admin info fail", zap.Error(err))
		return nil, err
	}
	a.user = user
	return a.user, nil
}

// ServiceOwner 服务(规则)负责人信息
type ServiceOwner struct {
	Namespace string
	Name      string
	Owners    map[string]bool
}

type LoginRequest struct {
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// ReverseProxyForLogin 反向代理
func ReverseProxyForLogin(PoleServer *bootstrap.PoleServer, conf *bootstrap.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del("Cookie")

		director := func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = PoleServer.Address
			req.Host = PoleServer.Address
			body, err := ioutil.ReadAll(req.Body)
			if err != nil {
				log.Error("[Proxy][Login] modify login request fail", zap.Error(err))
				return
			}

			admin, err := _adminGetter.GetAdminInfo()
			if err != nil {
				log.Error("[Proxy][Login] modify login request fail", zap.Error(err))
				return
			}

			loginBody := &LoginRequest{}
			_ = json.Unmarshal(body, loginBody)
			if len(admin.GetName()) != 0 {
				loginBody.Owner = admin.GetName()
			}
			body, err = json.Marshal(loginBody)
			if err != nil {
				log.Error("[Proxy][Login] modify login request fail", zap.Error(err))
				return
			}
			req.Header["Content-Length"] = []string{fmt.Sprint(len(body))}
			req.ContentLength = int64(len(body))
			req.Body = ioutil.NopCloser(bytes.NewBuffer(body))
		}
		modifyResp := func(resp *http.Response) error {
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			if err = resp.Body.Close(); err != nil {
				return err
			}
			loginResp := make(map[string]interface{})
			if err = json.Unmarshal(body, &loginResp); err != nil {
				return err
			}
			if val, ok := loginResp["loginResponse"].(map[string]interface{}); ok {
				if err = refreshJWTFromLoginPayload(c, val, conf); err != nil {
					return err
				}
				if token := val["token"]; token != "" {
					val["token"] = "******" // 避免前端出错,保证返回, 但隐藏现有的token
					body, err = json.Marshal(loginResp)
					if err != nil {
						return err
					}
					resp.Header["Content-Length"] = []string{fmt.Sprint(len(body))}
				}
				resp.Body = io.NopCloser(bytes.NewBuffer(body))
				return nil
			}
			if val, ok := loginResp["data"].(map[string]interface{}); ok {
				if err = refreshJWTFromLoginPayload(c, val, conf); err != nil {
					return err
				}
				resp.Body = io.NopCloser(bytes.NewBuffer(body))
				return nil
			}
			resp.Body = io.NopCloser(bytes.NewBuffer(body))
			return nil
		}
		proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: modifyResp, Transport: proxyTransport}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func refreshJWTFromLoginPayload(c *gin.Context, val map[string]interface{}, conf *bootstrap.Config) error {
	token, ok := val["token"].(string)
	if !ok || token == "" {
		return nil
	}
	userID, ok := val["user_id"].(string)
	if !ok || userID == "" {
		return nil
	}
	role, _ := val["role"].(string)
	return refreshJWTWithRole(c, userID, token, role, conf)
}

// ReverseProxyForServer 反向代理
func ReverseProxyForServer(PoleServer *bootstrap.PoleServer, conf *bootstrap.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, token, ok := verifyAccessPermission(c, conf)
		if !ok {
			return
		}
		// Preserve authorization attributes when extending the signed session.
		// The proxied request drops the browser cookie before it reaches the
		// control-plane, so the role must be captured first.
		role := ""
		if claims, err := parseJWTClaims(c, conf); err == nil && claims != nil {
			role = claims.Role
		}

		c.Request.Header.Del("Cookie")

		director := func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = PoleServer.Address
			req.Host = PoleServer.Address
		}
		modifyResp := func(resp *http.Response) error {
			if resp.StatusCode == http.StatusUnauthorized {
				expire, err := responseInvalidatesAuthentication(resp)
				if err != nil {
					return err
				}
				if expire {
					resp.Header.Add("Set-Cookie", expiredJWTCookie().String())
				}
				return nil
			}
			if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
				cookie, err := newJWTCookieWithRole(userID, token, role, conf)
				if err != nil {
					return err
				}
				if cookie != nil {
					resp.Header.Add("Set-Cookie", cookie.String())
				}
			}
			return nil
		}
		proxy := &httputil.ReverseProxy{Director: director, ModifyResponse: modifyResp, Transport: proxyTransport}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// responseInvalidatesAuthentication distinguishes an invalid credential from
// an authorization denial. HTTP 401 alone is ambiguous in the control-plane:
// Code_NotAllowedAccess means the signed Console session is still valid.
func responseInvalidatesAuthentication(resp *http.Response) (bool, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	if err = resp.Body.Close(); err != nil {
		return false, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))

	payload := struct {
		Code apimodel.Code `json:"code"`
	}{}
	if err = json.Unmarshal(body, &payload); err != nil {
		// Preserve the session when the backend did not provide an explicit
		// authentication code. A generic 401 must never imply logout.
		return false, nil
	}
	return payload.Code == apimodel.Code_TokenDisabled || payload.Code == apimodel.Code_TokenNotExisted, nil
}

func verifyAccessPermission(c *gin.Context, conf *bootstrap.Config) (string, string, bool) {
	return verifyAccessPermissionWithStatus(c, conf, http.StatusInternalServerError)
}

func verifyAccessPermissionWithStatus(c *gin.Context, conf *bootstrap.Config,
	status int) (string, string, bool) {
	userID, token, err := parseJWTThenSetToken(c, conf)
	if err != nil {
		c.JSON(status, gin.H{
			"code": http.StatusProxyAuthRequired,
			"info": "Proxy Authentication Required: " + err.Error(),
		})
		return "", "", false
	}

	if ok := checkAuthoration(c, conf); !ok {
		c.JSON(status, gin.H{
			"code": http.StatusProxyAuthRequired,
			"info": "Proxy Authentication Required: access token is invalid",
		})
		return "", "", false
	}

	return userID, token, true
}

// ReverseProxyNoAuthForServer 反向代理
func ReverseProxyNoAuthForServer(PoleServer *bootstrap.PoleServer, conf *bootstrap.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Header.Del("Cookie")

		director := func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = PoleServer.Address
			req.Host = PoleServer.Address
		}
		proxy := &httputil.ReverseProxy{Director: director, Transport: proxyTransport}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

func ReverseProxyForMonitorServer(monitorServer *bootstrap.MonitorServer) gin.HandlerFunc {
	return func(c *gin.Context) {

		director := func(req *http.Request) {
			req.URL.Scheme = "http"
			req.URL.Host = monitorServer.Address
			req.Host = monitorServer.Address
		}
		proxy := &httputil.ReverseProxy{Director: director, Transport: proxyTransport}
		proxy.ServeHTTP(c.Writer, c.Request)
	}
}

// jwtClaims jwt 额外信息
type jwtClaims struct {
	UserID string
	Token  string
	Role   string
	jwt.RegisteredClaims
}

func parseJWTClaims(c *gin.Context, conf *bootstrap.Config) (*jwtClaims, error) {
	if c == nil || c.Request == nil {
		return nil, nil
	}
	jwtCookie, _ := c.Request.Cookie("jwt")
	if jwtCookie == nil {
		return nil, nil
	}
	token, err := jwt.ParseWithClaims(jwtCookie.Value, &jwtClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(conf.WebServer.JWT.SecretKey), nil
	})
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*jwtClaims)
	if !ok || !token.Valid || claims.UserID == "" {
		return nil, errors.New("jwt token is invalid")
	}
	return claims, nil
}

// parseJWTThenSetToken 从jwt中抽取userID 和 token
func parseJWTThenSetToken(c *gin.Context, conf *bootstrap.Config) (string, string, error) {
	receiveUserId := c.Request.Header.Get("x-pole-user")

	claims, err := parseJWTClaims(c, conf)
	if claims == nil && err == nil {
		return "", "", nil
	}
	if err != nil {
		log.Error("parse jwt with claims fail", zap.Error(err))
		return "", "", err
	}
	if receiveUserId != claims.UserID {
		log.Error("Login information comparison failed", zap.String("receive-user-id", receiveUserId), zap.String("jwt-user-id", claims.UserID))
		return "", "", errors.New("Login information comparison failed. Maybe the login information came from illegal injection.")
	}

	c.Request.Header.Set("x-pole-user", claims.UserID)
	c.Request.Header.Set("Authorization", claims.Token)
	return claims.UserID, claims.Token, nil
}

// refreshJWT 刷新jwtToken
func refreshJWT(c *gin.Context, userID, token string, conf *bootstrap.Config) error {
	role := ""
	if claims, err := parseJWTClaims(c, conf); err == nil && claims != nil {
		role = claims.Role
	}
	return refreshJWTWithRole(c, userID, token, role, conf)
}

func refreshJWTWithRole(c *gin.Context, userID, token, role string, conf *bootstrap.Config) error {
	cookie, err := newJWTCookieWithRole(userID, token, role, conf)
	if err != nil || cookie == nil {
		return err
	}
	http.SetCookie(c.Writer, cookie)
	return nil
}

func newJWTCookie(userID, token string, conf *bootstrap.Config) (*http.Cookie, error) {
	return newJWTCookieWithRole(userID, token, "", conf)
}

func newJWTCookieWithRole(userID, token, role string, conf *bootstrap.Config) (*http.Cookie, error) {
	if userID == "" || token == "" {
		return nil, nil
	}
	nowTime := time.Now()
	claims := jwtClaims{
		UserID: userID,
		Token:  token,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(nowTime.Add(time.Duration(conf.WebServer.JWT.Expired) * time.Second)),
			NotBefore: jwt.NewNumericDate(nowTime),
			IssuedAt:  jwt.NewNumericDate(nowTime),
		},
	}
	jwtToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(conf.WebServer.JWT.SecretKey))
	if err != nil {
		return nil, err
	}
	return &http.Cookie{
		Name:     "jwt",
		Value:    jwtToken,
		Path:     "/",
		MaxAge:   conf.WebServer.JWT.Expired,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}, nil
}

func expiredJWTCookie() *http.Cookie {
	return &http.Cookie{
		Name:    "jwt",
		Value:   "",
		Path:    "/",
		MaxAge:  -1,
		Expires: time.Unix(0, 0),
	}
}
