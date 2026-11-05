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

package job

import (
	"testing"
	"time"

	"github.com/golang/mock/gomock"
	apiservice "github.com/pole-io/specification/source/go/api/v1/service_manage"

	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	svctypes "github.com/pole-io/pole-server/apis/pkg/types/service"
	storemock "github.com/pole-io/pole-server/plugin/store/mock"
)

func Test_DeleteUnHealthyInstanceJobConfigInit(t *testing.T) {
	expectValue := 10 * time.Minute
	expectSweepInterval := 30 * time.Second
	raw := map[string]interface{}{
		"instanceDeleteTimeout": "10m",
		"sweepInterval":         "30s",
	}

	job := deleteUnHealthyInstanceJob{}
	err := job.init(raw)
	if err != nil {
		t.Errorf("init deleteUnHealthyInstanceJob config, err: %v", err)
	}

	if job.cfg.InstanceDeleteTimeout != expectValue {
		t.Errorf("init deleteUnHealthyInstanceJob config. expect: %s, actual: %s",
			expectValue, job.cfg.InstanceDeleteTimeout)
	}
	if job.interval() != expectSweepInterval {
		t.Errorf("init sweep interval. expect: %s, actual: %s",
			expectSweepInterval, job.interval())
	}
}

func Test_DeleteUnHealthyInstanceJobConfigDefaultsSweepInterval(t *testing.T) {
	job := deleteUnHealthyInstanceJob{}
	if err := job.init(map[string]interface{}{}); err != nil {
		t.Fatalf("init default config: %v", err)
	}
	if job.interval() != time.Minute {
		t.Fatalf("default sweep interval = %s", job.interval())
	}
}

func Test_DeleteUnHealthyInstanceJobConfigInitErr(t *testing.T) {
	raw := map[string]interface{}{
		"instanceDeleteTimeout": "xx",
	}

	job := deleteUnHealthyInstanceJob{}
	err := job.init(raw)
	if err == nil {
		t.Errorf("init deleteUnHealthyInstanceJob config should err")
	}
}

func TestGetMasterAccountTokenUsesCanonicalMainUser(t *testing.T) {
	ctrl := gomock.NewController(t)
	storage := storemock.NewMockStore(ctrl)
	storage.EXPECT().GetMainUser().Return(&authtypes.User{Name: "admin", Token: "main-token"}, nil)

	token, err := getMasterAccountToken(storage)
	if err != nil {
		t.Fatalf("get main account token: %v", err)
	}
	if token != "main-token" {
		t.Fatalf("token = %q", token)
	}
}

func TestUnhealthyInstanceDeleteRequestIncludesIdentityTetrad(t *testing.T) {
	request := unhealthyInstanceDeleteRequest(&svctypes.Instance{Proto: &apiservice.Instance{
		Id: "instance-id", Namespace: "pole-system", Service: "pole.checker",
		Host: "10.0.0.8", Port: 8091,
	}})
	if request.GetId() != "instance-id" || request.GetNamespace() != "pole-system" ||
		request.GetService() != "pole.checker" || request.GetHost() != "10.0.0.8" || request.GetPort() != 8091 {
		t.Fatalf("incomplete delete identity: %+v", request)
	}
}
