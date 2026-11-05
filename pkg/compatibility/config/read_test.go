package config

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

type discoverFunc func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response

func (f discoverFunc) DiscoverForProtocol(ctx context.Context, req *configv1.DiscoverRequest, protocol string) *apimodel.Response {
	response := f(ctx, req, protocol)
	if response.Data != nil {
		result := &configv1.ConfigResult{}
		if response.Data.UnmarshalTo(result) == nil {
			result.Key = req.Key
			if result.Presence == configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT {
				result.ReleaseId = "release"
			}
			result.SnapshotRevision = "snapshot"
			result.ObservationCursor = &configv1.Cursor{Scope: "config", Epoch: "epoch"}
			response.Data, _ = anypb.New(result)
		}
	}
	return response
}

func TestReadNeverCachesOrConvertsFailureToAbsence(t *testing.T) {
	calls := 0
	service := discoverFunc(func(ctx context.Context, req *configv1.DiscoverRequest, protocol string) *apimodel.Response {
		calls++
		if protocol != "nacos" {
			t.Fatal(protocol)
		}
		if calls > 1 {
			return &apimodel.Response{Code: uint32(apimodel.Code_ClientAPINotOpen)}
		}
		data, _ := anypb.New(&configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT, Snapshot: &configv1.RenderSnapshot{Engine: "literal"}})
		return &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}
	})
	first, err := Read(context.Background(), service, "nacos", &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: "e", Group: "g", Name: "n"}})
	if err != nil || first.MD5 != "d41d8cd98f00b204e9800998ecf8427e" {
		t.Fatal(first, err)
	}
	second, err := Read(context.Background(), service, "nacos", &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: "e", Group: "g", Name: "n"}})
	var failure *Error
	if second != nil || !errors.As(err, &failure) || failure.Code != apimodel.Code_ClientAPINotOpen {
		t.Fatal("disabled result reused", second, err)
	}
}

func TestReadRenderFailureAndAbsent(t *testing.T) {
	for _, test := range []struct {
		name   string
		result *configv1.ConfigResult
		code   apimodel.Code
	}{
		{"absent", &configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT}, 0},
		{"missing-snapshot", &configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT}, apimodel.Code_ConfigUnavailable},
		{"invalid-json", &configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT, Snapshot: &configv1.RenderSnapshot{Engine: "pole-mustache-v1", Content: []byte("{"), Format: "json"}}, apimodel.Code_ConfigRenderFailed},
		{"absent-with-content", &configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_ABSENT, Snapshot: &configv1.RenderSnapshot{}}, apimodel.Code_ConfigUnavailable},
	} {
		t.Run(test.name, func(t *testing.T) {
			data, _ := anypb.New(test.result)
			service := discoverFunc(func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response {
				return &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}
			})
			result, err := Read(context.Background(), service, "apollo", &configv1.DiscoverRequest{Key: &configv1.ConfigKey{Environment: "e", Group: "g", Name: "n"}})
			if test.code == 0 {
				if err != nil || result.MD5 != "" {
					t.Fatal(result, err)
				}
				return
			}
			var failure *Error
			if !errors.As(err, &failure) || failure.Code != test.code {
				t.Fatal(err)
			}
		})
	}
}

func TestWaitTerminalRecheckAndCancellation(t *testing.T) {
	calls := 0
	err := Wait(context.Background(), time.Nanosecond, func(context.Context) (bool, error) {
		calls++
		if calls > 1 {
			return false, &Error{Code: apimodel.Code_ClientAPINotOpen}
		}
		return false, nil
	})
	if err == nil || calls != 2 {
		t.Fatal(calls, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Wait(ctx, time.Minute, func(context.Context) (bool, error) { return false, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

type rawDiscover struct{ response *apimodel.Response }

func (service rawDiscover) DiscoverForProtocol(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response {
	return service.response
}

func TestMalformedSuccessIsUnavailable(t *testing.T) {
	key := &configv1.ConfigKey{Environment: "e", Group: "g", Name: "n"}
	for _, test := range []struct {
		name   string
		mutate func(*configv1.ConfigResult)
	}{
		{"missing-key", func(r *configv1.ConfigResult) { r.Key = nil }},
		{"wrong-key", func(r *configv1.ConfigResult) {
			r.Key = &configv1.ConfigKey{Environment: "other", Group: "g", Name: "n"}
		}},
		{"missing-cursor", func(r *configv1.ConfigResult) { r.ObservationCursor = nil }},
		{"missing-epoch", func(r *configv1.ConfigResult) { r.ObservationCursor.Epoch = "" }},
		{"missing-release", func(r *configv1.ConfigResult) { r.ReleaseId = "" }},
		{"missing-revision", func(r *configv1.ConfigResult) { r.SnapshotRevision = "" }},
		{"unknown-presence", func(r *configv1.ConfigResult) { r.Presence = 99 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := &configv1.ConfigResult{Key: key, Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT, ReleaseId: "r", SnapshotRevision: "s", ObservationCursor: &configv1.Cursor{Scope: "config", Epoch: "epoch"}, Snapshot: &configv1.RenderSnapshot{Engine: "literal"}}
			test.mutate(result)
			data, _ := anypb.New(result)
			got, err := Read(context.Background(), rawDiscover{&apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}}, "nacos", &configv1.DiscoverRequest{Key: key})
			var failure *Error
			if got != nil || !errors.As(err, &failure) || failure.Code != apimodel.Code_ConfigUnavailable {
				t.Fatal(got, err)
			}
		})
	}
}
