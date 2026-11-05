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

package model

import (
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/emicklei/go-restful/v3"
)

type ConfigFileBase struct {
	Namespace string `param:"tenant"`
	Group     string `param:"group"`
	DataId    string `param:"dataId"`
}

type ConfigFile struct {
	ConfigFileBase
	Content          string `param:"content"`
	Tag              string `param:"tag"`
	AppName          string `param:"appName"`
	BetaIps          string `param:"betaIps"`
	CasMd5           string `param:"casMd5"`
	Type             string `param:"type"`
	SrcUser          string `param:"srcUser"`
	Labels           string `param:"config_tags"`
	Description      string `param:"desc"`
	EncryptedDataKey string `param:"encryptedDataKey"`
}

const (
	LineSeparatorRune = '\x01'
	WordSeparatorRune = '\x02'
)

var (
	InvalidProbeModify = &NacosError{
		ErrCode: int32(http.StatusBadRequest),
		ErrMsg:  "invalid probeModify",
	}
)

type ConfigWatchContext struct {
	Request       *restful.Request
	ClientVersion string
	Items         []*ConfigListenItem
}

func (cw *ConfigWatchContext) IsSupportLongPolling() (time.Duration, bool) {
	val := cw.Request.HeaderParameter("Long-Pulling-Timeout")
	if val == "" {
		return 0, false
	}
	timeoutMs, err := strconv.ParseInt(val, 10, 64)
	if err != nil {
		return 0, false
	}

	finalTimeout := int64(math.Max(10000, float64(timeoutMs-2000)))
	return time.Millisecond * time.Duration(finalTimeout), true
}

func (cw *ConfigWatchContext) IsNoHangUp() bool {
	val := cw.Request.HeaderParameter("Long-Pulling-Timeout-No-Hangup")
	return strings.Compare(val, "true") == 0
}

type ConfigListenItem struct {
	Tenant string `json:"tenant"`
	Group  string `json:"group"`
	DataId string `json:"dataId"`
	Md5    string `json:"md5"`
}

func (c *ConfigListenItem) Key() string {
	if c.Tenant == "" {
		return url.QueryEscape(c.DataId + "+" + c.Group)
	}
	return url.QueryEscape(c.DataId + "+" + c.Group + "+" + c.Tenant)
}

// ParseConfigListenContext parse the transport protocol, which has two formats (W for field delimiter, L for each data delimiter)
// old: D w G w MD5 l
// new: D w G w MD5 w T l.
func ParseConfigListenContext(req *restful.Request, configKeysString string) (*ConfigWatchContext, error) {
	watchCtx := &ConfigWatchContext{
		Request: req,
		Items:   make([]*ConfigListenItem, 0, 4),
	}
	if configKeysString == "" {
		return watchCtx, nil
	}

	start := 0
	tmpList := make([]string, 0, 3)
	for i := start; i < len(configKeysString); i++ {
		c := configKeysString[i]
		if c == WordSeparatorRune {
			tmpList = append(tmpList, configKeysString[start:i])
			start = i + 1
			if len(tmpList) > 3 {
				// Malformed message and return parameter error.
				return nil, InvalidProbeModify
			}
		} else if c == LineSeparatorRune {
			endValue := ""
			if start+1 <= i {
				endValue = configKeysString[start:i]
			}
			start = i + 1

			// If it is the old message, the last digit is MD5. The post-multi-tenant message is tenant
			if len(tmpList) != 2 && len(tmpList) != 3 {
				return nil, InvalidProbeModify
			}
			if len(tmpList) == 2 {
				watchCtx.Items = append(watchCtx.Items, &ConfigListenItem{
					Group:  tmpList[1],
					DataId: tmpList[0],
					Md5:    endValue,
				})
			} else {
				watchCtx.Items = append(watchCtx.Items, &ConfigListenItem{
					Tenant: endValue,
					Group:  tmpList[1],
					DataId: tmpList[0],
					Md5:    tmpList[2],
				})
			}
			tmpList = tmpList[:0]
			// Protect malformed messages
			if len(watchCtx.Items) > 128 {
				return nil, InvalidProbeModify
			}
		}
	}

	if start != len(configKeysString) {
		return nil, InvalidProbeModify
	}
	watchCtx.ClientVersion = req.HeaderParameter("Client-Version")
	if watchCtx.ClientVersion == "" {
		watchCtx.ClientVersion = "2.0.0"
	}

	return watchCtx, nil
}
