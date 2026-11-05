package v1

import (
	"context"
	"errors"
	"io"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"

	"github.com/pole-io/pole-server/apis/store"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
	"github.com/pole-io/pole-server/pkg/config/unified"
)

type configFrameStream struct {
	ctx                  context.Context
	requests             []*configv1.DiscoverRequest
	sent                 []*apimodel.Response
	recvError, sendError error
}

func (s *configFrameStream) SetHeader(metadata.MD) error { return nil }

func (s *configFrameStream) SendHeader(metadata.MD) error { return nil }

func (s *configFrameStream) SetTrailer(metadata.MD) {}

func (s *configFrameStream) Context() context.Context { return s.ctx }

func (s *configFrameStream) SendMsg(interface{}) error { return nil }

func (s *configFrameStream) RecvMsg(interface{}) error { return nil }

func (s *configFrameStream) Recv() (*configv1.DiscoverRequest, error) {
	if len(s.requests) == 0 {
		if s.recvError != nil {
			return nil, s.recvError
		}
		return nil, io.EOF
	}
	request := s.requests[0]
	s.requests = s.requests[1:]
	return request, nil
}

func (s *configFrameStream) Send(response *apimodel.Response) error {
	if s.sendError != nil {
		return s.sendError
	}
	s.sent = append(s.sent, response)
	return nil
}

type denyConfigFrames struct{}

func (denyConfigFrames) Authorize(context.Context, *configv1.ConfigKey, unified.Action) (string, error) {
	return "", unified.ErrAccessDenied
}

type unusedConfigFrameStore struct{ store.UnifiedConfigStore }

func TestConfigDiscoverFrameGatesAndIOErrors(t *testing.T) {
	core, err := unified.New(unusedConfigFrameStore{}, denyConfigFrames{})
	if err != nil {
		t.Fatal(err)
	}
	key := &configv1.ConfigKey{Environment: "prod", Group: "app", Name: "config"}
	for _, test := range []struct {
		name  string
		gate  bool
		limit uint32
		code  codes.Code
	}{
		{"disabled", false, uint32(api.ExecuteSuccess), codes.Unimplemented},
		{"rate limited", true, 0, codes.ResourceExhausted},
	} {
		t.Run(test.name, func(t *testing.T) {
			checked, limited := 0, 0
			server := NewConfigGRPCServer(WithUnifiedConfigServer(core), WithCAllowAccess(func(method string) bool {
				checked++
				if method != "/v1.ConfigGRPC/Discover" {
					t.Fatal(method)
				}
				return checked == 1 || test.gate
			}), WithCEnterRateLimit(func(_, method string) uint32 {
				limited++
				if limited == 1 {
					return uint32(api.ExecuteSuccess)
				}
				return test.limit
			}))
			stream := &configFrameStream{ctx: context.Background(), requests: []*configv1.DiscoverRequest{{Key: key}, {Key: key}}}
			if err := server.Discover(stream); status.Code(err) != test.code {
				t.Fatalf("frame gate: %v", err)
			}
			if checked != 2 || len(stream.sent) != 1 || (test.gate && limited != 2) {
				t.Fatalf("gate not per frame: checked=%d limit=%d sent=%d", checked, limited, len(stream.sent))
			}
		})
	}
	for _, send := range []bool{false, true} {
		failure := errors.New("transport failure")
		stream := &configFrameStream{ctx: context.Background()}
		if send {
			stream.requests = []*configv1.DiscoverRequest{{Key: key}}
			stream.sendError = failure
		} else {
			stream.recvError = failure
		}
		server := NewConfigGRPCServer(WithUnifiedConfigServer(core))
		if err := server.Discover(stream); !errors.Is(err, failure) {
			t.Fatalf("transport failure swallowed: %v", err)
		}
	}
	missing := NewConfigGRPCServer()
	if err := missing.Discover(&configFrameStream{ctx: context.Background(), requests: []*configv1.DiscoverRequest{{Key: key}}}); status.Code(err) != codes.Unavailable {
		t.Fatalf("missing dependency: %v", err)
	}
}

func TestConfigDiscoverBusinessFailureAllowsFollowingFrame(t *testing.T) {
	core, err := unified.New(unusedConfigFrameStore{}, denyConfigFrames{})
	if err != nil {
		t.Fatal(err)
	}
	key := &configv1.ConfigKey{Environment: "prod", Group: "app", Name: "config"}
	stream := &configFrameStream{ctx: context.Background(), requests: []*configv1.DiscoverRequest{{Key: key}, {Key: key}}}
	if err := NewConfigGRPCServer(WithUnifiedConfigServer(core)).Discover(stream); err != nil {
		t.Fatal(err)
	}
	if len(stream.sent) != 2 {
		t.Fatalf("received %d responses for 2 requests", len(stream.sent))
	}
	for i, response := range stream.sent {
		if response.GetCode() == uint32(api.ExecuteSuccess) || response.GetData() != nil {
			t.Fatalf("frame %d did not contain a public business failure: %v", i, response)
		}
	}
}
