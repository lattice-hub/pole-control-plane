/*
 * Copyright 1999-2020 Alibaba Group Holding Ltd.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package nacos_grpc_service

import (
	"github.com/pole-io/pole-server/pkg/common/utils"
)

type ConfigListenContext struct {
	Group  string `json:"group"`
	Md5    string `json:"md5"`
	DataId string `json:"dataId"`
	Tenant string `json:"tenant"`
}

type ConfigContext struct {
	Group  string `json:"group"`
	DataId string `json:"dataId"`
	Tenant string `json:"tenant"`
}

type ConfigRequest struct {
	*Request
	Group  string `json:"group"`
	DataId string `json:"dataId"`
	Tenant string `json:"tenant"`
	Module string `json:"module"`
}

func (c *ConfigRequest) RequestMeta() interface{} {
	return c
}

func NewConfigRequest() *ConfigRequest {
	request := Request{
		RequestId: utils.NewUUID(),
		Headers:   make(map[string]string, 8),
	}
	return &ConfigRequest{
		Request: &request,
		Module:  "config",
	}
}

func (r *ConfigRequest) GetDataId() string {
	return r.DataId
}

func (r *ConfigRequest) GetGroup() string {
	return r.Group
}

func (r *ConfigRequest) GetTenant() string {
	return r.Tenant
}

// ConfigBatchListenRequest request of listening a batch of configs.
type ConfigBatchListenRequest struct {
	*ConfigRequest
	Listen               bool                  `json:"listen"`
	ConfigListenContexts []ConfigListenContext `json:"configListenContexts"`
}

func (c *ConfigBatchListenRequest) RequestMeta() interface{} {
	return c
}

func NewConfigBatchListenRequest() *ConfigBatchListenRequest {
	return &ConfigBatchListenRequest{
		Listen:               true,
		ConfigListenContexts: make([]ConfigListenContext, 0, 4),
		ConfigRequest:        NewConfigRequest(),
	}
}

func (r *ConfigBatchListenRequest) GetRequestType() string {
	return "ConfigBatchListenRequest"
}

type ConfigChangeNotifyRequest struct {
	*ConfigRequest
}

func NewConfigChangeNotifyRequest() *ConfigChangeNotifyRequest {
	return &ConfigChangeNotifyRequest{
		ConfigRequest: NewConfigRequest(),
	}
}

func (r *ConfigChangeNotifyRequest) GetRequestType() string {
	return "ConfigChangeNotifyRequest"
}

type ConfigQueryRequest struct {
	*ConfigRequest
	Tag string `json:"tag"`
}

func (c *ConfigQueryRequest) RequestMeta() interface{} {
	return c
}

func NewConfigQueryRequest() *ConfigQueryRequest {
	return &ConfigQueryRequest{ConfigRequest: NewConfigRequest()}
}

func (r *ConfigQueryRequest) GetRequestType() string {
	return "ConfigQueryRequest"
}

type ConfigPublishRequest struct {
	*ConfigRequest
	Content     string            `json:"content"`
	CasMd5      string            `json:"casMd5"`
	AdditionMap map[string]string `json:"additionMap"`
}

func (c *ConfigPublishRequest) RequestMeta() interface{} {
	return c
}

func NewConfigPublishRequest() *ConfigPublishRequest {
	return &ConfigPublishRequest{
		ConfigRequest: NewConfigRequest(),
		AdditionMap:   make(map[string]string),
	}
}

func (r *ConfigPublishRequest) GetRequestType() string {
	return "ConfigPublishRequest"
}

type ConfigRemoveRequest struct {
	*ConfigRequest
}

func (c *ConfigRemoveRequest) RequestMeta() interface{} {
	return c
}

func NewConfigRemoveRequest() *ConfigRemoveRequest {
	return &ConfigRemoveRequest{ConfigRequest: NewConfigRequest()}
}

func (r *ConfigRemoveRequest) GetRequestType() string {
	return "ConfigRemoveRequest"
}
