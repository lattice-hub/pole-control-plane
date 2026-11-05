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

package utils

import (
	"testing"

	"github.com/pole-io/pole-server/apis/apiserver"
)

func TestGetClientOpenMethod(t *testing.T) {
	type args struct {
		include  []string
		protocol string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]bool
		wantErr bool
	}{
		{
			name: "register access includes legacy and spec service names",
			args: args{
				include: []string{
					apiserver.RegisterAccess,
				},
				protocol: "grpc",
			},
			want: map[string]bool{
				"/v1.PolarisGRPC/RegisterInstance":    true,
				"/v1.PolarisGRPC/DeregisterInstance":  true,
				"/v1.DiscoverGRPC/RegisterInstance":   true,
				"/v1.DiscoverGRPC/DeregisterInstance": true,
			},
			wantErr: false,
		},
		{
			name: "discover access includes legacy and spec service names",
			args: args{
				include: []string{
					apiserver.DiscoverAccess,
				},
				protocol: "grpc",
			},
			want: map[string]bool{
				"/v1.PolarisGRPC/Discover":                             true,
				"/v1.PolarisGRPC/ReportClient":                         true,
				"/v1.PolarisServiceContractGRPC/ReportServiceContract": true,
				"/v1.PolarisServiceContractGRPC/GetServiceContract":    true,
				"/v1.DiscoverGRPC/Discover":                            true,
				"/v1.DiscoverGRPC/ReportClient":                        true,
				"/v1.DiscoverGRPC/ReportServiceContract":               true,
				"/v1.DiscoverGRPC/GetServiceContract":                  true,
			},
			wantErr: false,
		},
		{
			name: "healthcheck access includes legacy and spec service names",
			args: args{
				include: []string{
					apiserver.HealthcheckAccess,
				},
				protocol: "grpc",
			},
			want: map[string]bool{
				"/v1.PolarisGRPC/Heartbeat":                  true,
				"/v1.PolarisHeartbeatGRPC/BatchHeartbeat":    true,
				"/v1.PolarisHeartbeatGRPC/BatchGetHeartbeat": true,
				"/v1.PolarisHeartbeatGRPC/BatchDelHeartbeat": true,
				"/v1.DiscoverGRPC/Heartbeat":                 true,
				"/v1.PoleHeartbeatGRPC/BatchGetHeartbeat":    true,
				"/v1.PoleHeartbeatGRPC/BatchDelHeartbeat":    true,
			},
			wantErr: false,
		},
		{
			name: "config access includes only Discover",
			args: args{
				include: []string{
					apiserver.ConfigAccess,
				},
				protocol: "grpc",
			},
			want: map[string]bool{
				"/v1.ConfigGRPC/Discover": true,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetDiscoverClientOpenMethod(tt.args.include, tt.args.protocol)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDiscoverClientOpenMethod() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			for method := range tt.want {
				if !got[method] {
					t.Errorf("GetDiscoverClientOpenMethod() missing method %s, got %v", method, got)
				}
			}
		})
	}
}

func TestConfigClientOpenMethodContainsOnlyRuntimeRPCs(t *testing.T) {
	methods, err := GetDiscoverClientOpenMethod([]string{apiserver.ConfigAccess}, "grpc")
	if err != nil {
		t.Fatal(err)
	}
	if len(methods) != 1 {
		t.Fatalf("unexpected config methods: %v", methods)
	}
	for _, method := range []string{"Discover"} {
		if !methods["/v1.ConfigGRPC/"+method] {
			t.Fatalf("missing runtime method %s", method)
		}
	}
	standalone, err := GetConfigClientOpenMethod("grpc")
	if err != nil || len(standalone) != 1 {
		t.Fatalf("standalone config exposes other methods: %v %v", standalone, err)
	}
	for _, method := range []string{"Resolve", "Watch", "SaveDraft", "GetState", "Preview", "Publish", "Stop", "StopAllGray", "Promote", "Rollback", "Offline", "CreateConfigFile", "UpdateConfigFile", "PublishConfigFile", "PreviewConfigTemplate", "UpsertAndPublishConfigFile"} {
		if methods["/v1.ConfigGRPC/"+method] || standalone["/v1.ConfigGRPC/"+method] {
			t.Fatalf("management method is open: %s", method)
		}
	}
}
