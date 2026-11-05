package config

import (
	"context"
	"io"
	"sync/atomic"
	"testing"
	"time"

	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/anypb"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/pkg/types"
	nacospb "github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v2/pb"
	"github.com/pole-io/pole-server/plugin/apiserver/nacosserver/v2/remote"
)

type fakeDiscover func(context.Context, *configv1.DiscoverRequest, string) *apimodel.Response

func (f fakeDiscover) DiscoverForProtocol(ctx context.Context, request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
	response := f(ctx, request, protocol)
	if response.Data != nil {
		result := &configv1.ConfigResult{}
		if response.Data.UnmarshalTo(result) == nil {
			result.Key = request.Key
			result.ReleaseId = "release"
			if result.SnapshotRevision == "" {
				result.SnapshotRevision = "snapshot"
			}
			if result.ObservationCursor == nil {
				result.ObservationCursor = &configv1.Cursor{}
			}
			result.ObservationCursor.Scope = "config"
			result.ObservationCursor.Epoch = "epoch"
			response.Data, _ = anypb.New(result)
		}
	}
	return response
}

func success(content []byte) *apimodel.Response {
	data, _ := anypb.New(&configv1.ConfigResult{Presence: configv1.ConfigPresence_CONFIG_PRESENCE_PRESENT, Snapshot: &configv1.RenderSnapshot{Engine: "literal", Content: content, Format: "text"}})
	return &apimodel.Response{Code: uint32(apimodel.Code_ExecuteSuccess), Data: data}
}

func TestNacosV2ReadLabelsChecksumAndBinaryRefusal(t *testing.T) {
	var binary bool
	server := &ConfigServer{configSvr: fakeDiscover(func(ctx context.Context, request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
		if protocol != "nacos" || request.Labels["region"] != "cn" || request.Labels[types.ClientLabel_IP] != "127.0.0.1" {
			t.Fatal(request, protocol)
		}
		if binary {
			return success([]byte{255})
		}
		return success([]byte{})
	})}
	query := nacospb.NewConfigQueryRequest()
	query.Tenant = "dev"
	query.Group = "g"
	query.DataId = "n"
	meta := nacospb.RequestMeta{ClientIP: "127.0.0.1", Labels: map[string]string{"region": "cn"}}
	response, err := server.handleGetConfigRequest(context.Background(), query, meta)
	if err != nil {
		t.Fatal(err)
	}
	result := response.(*nacospb.ConfigQueryResponse)
	if !result.Success || result.Md5 != "d41d8cd98f00b204e9800998ecf8427e" || result.Content != "" {
		t.Fatal(result)
	}
	binary = true
	response, err = server.handleGetConfigRequest(context.Background(), query, meta)
	if err != nil || response.(*nacospb.ConfigQueryResponse).ErrorCode != int(apimodel.Code_ConfigRenderFailed) {
		t.Fatal(response, err)
	}
	response, err = server.handlePublishConfigRequest(context.Background(), nacospb.NewConfigPublishRequest(), meta)
	if err != nil || response.(*nacospb.ConfigPublishResponse).ErrorCode != 405 {
		t.Fatal(response, err)
	}
}

type fakeStream struct {
	ctx    context.Context
	cancel context.CancelFunc
	sends  atomic.Int32
}

func (s *fakeStream) SetHeader(metadata.MD) error { return nil }

func (s *fakeStream) SendHeader(metadata.MD) error { return nil }

func (s *fakeStream) SetTrailer(metadata.MD) {}

func (s *fakeStream) Context() context.Context { return s.ctx }

func (s *fakeStream) SendMsg(any) error { s.sends.Add(1); s.cancel(); return nil }

func (s *fakeStream) RecvMsg(any) error { return io.EOF }

func TestNacosV2PollReauthorizesAndDoesNotNotifyFailure(t *testing.T) {
	for _, reject := range []bool{false, true} {
		t.Run(map[bool]string{false: "change", true: "disabled"}[reject], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			stream := &fakeStream{ctx: ctx, cancel: cancel}
			var calls atomic.Int32
			server := &ConfigServer{configSvr: fakeDiscover(func(readCtx context.Context, request *configv1.DiscoverRequest, protocol string) *apimodel.Response {
				calls.Add(1)
				if readCtx.Value(types.ContextAuthTokenKey) != "credential" {
					t.Error("credential lost")
				}
				if reject {
					cancel()
					return &apimodel.Response{Code: uint32(apimodel.Code_ClientAPINotOpen)}
				}
				return success([]byte("changed"))
			})}
			item := nacospb.ConfigListenContext{Tenant: "dev", Group: "g", DataId: "n", Md5: "old"}
			sub := &subscription{ctx: context.WithValue(context.Background(), types.ContextAuthTokenKey, "credential"), items: map[nacospb.ConfigContext]nacospb.ConfigListenContext{interestKey(item): item}, cancel: cancel}
			server.watches.Store("connection", sub)
			done := make(chan struct{})
			go func() {
				defer close(done)
				server.poll(ctx, "connection", sub, &remote.SyncServerStream{Stream: stream})
			}()
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("poll did not stop")
			}
			want := int32(1)
			if reject {
				want = 0
			}
			if calls.Load() != 1 || stream.sends.Load() != want {
				t.Fatal(calls.Load(), stream.sends.Load())
			}
			if _, ok := server.watches.Load("connection"); ok {
				t.Fatal("subscription leaked")
			}
		})
	}
}

func TestNacosV2StopCancelsSubscriptions(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &ConfigServer{}
	server.watches.Store("id", &subscription{cancel: cancel})
	server.Stop()
	if ctx.Err() != context.Canceled {
		t.Fatal("subscription retained")
	}
}
