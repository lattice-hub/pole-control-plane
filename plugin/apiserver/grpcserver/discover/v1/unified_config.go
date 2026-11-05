package v1

import (
	"context"
	"io"
	"net"
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	apiconfig "github.com/pole-io/specification/source/go/api/v1/config_manage"

	"github.com/pole-io/pole-server/apis/pkg/types"
	api "github.com/pole-io/pole-server/pkg/common/api/v1"
)

var _ apiconfig.ConfigGRPCServer = (*ConfigGRPCServer)(nil)

// unifiedConfigContext 保留原请求取消、deadline 与 tracing；metadata 仅传递凭据，不接受声明身份。
func unifiedConfigContext(ctx context.Context) context.Context {
	incoming, _ := metadata.FromIncomingContext(ctx)
	headers := incoming.Copy()
	headers.Delete("staffname")
	ctx = types.AppendRequestHeader(ctx, headers)
	ctx = context.WithValue(ctx, types.ContextGrpcHeader, headers)
	for _, binding := range []struct {
		header string
		key    types.StringContext
	}{
		{strings.ToLower(types.HeaderAuthorizationKey), types.ContextAuthTokenKey},
		{strings.ToLower(types.HeaderRequestId), types.ContextRequestId},
		{"user-agent", types.ContextUserAgent},
	} {
		if values := headers.Get(binding.header); len(values) > 0 {
			ctx = context.WithValue(ctx, binding.key, values[0])
		}
	}
	if remote, ok := peer.FromContext(ctx); ok && remote.Addr != nil {
		address := remote.Addr.String()
		ctx = context.WithValue(ctx, types.ContextClientAddress, address)
		if host, _, err := net.SplitHostPort(address); err == nil {
			ctx = context.WithValue(ctx, types.ContextClientIP, host)
		}
	}
	return ctx
}

// Discover 对每帧请求重新鉴权和读取，仅返回该请求的完整最终结果。
func (g *ConfigGRPCServer) Discover(stream apiconfig.ConfigGRPC_DiscoverServer) error {
	ctx := unifiedConfigContext(stream.Context())
	const method = "/v1.ConfigGRPC/Discover"
	clientIP, _ := ctx.Value(types.ContextClientIP).(string)
	for {
		request, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		if g.allowAccess != nil && !g.allowAccess(method) {
			return status.Error(codes.Unimplemented, "configuration client API is not enabled")
		}
		if g.enterRateLimit != nil && g.enterRateLimit(clientIP, method) != uint32(api.ExecuteSuccess) {
			return status.Error(codes.ResourceExhausted, "configuration request rate limit exceeded")
		}
		if g.unifiedService == nil {
			return status.Error(codes.Unavailable, "unified configuration is unavailable")
		}
		response := g.unifiedService.Discover(ctx, request)
		if err := ctx.Err(); err != nil {
			return status.FromContextError(err).Err()
		}
		if err := stream.Send(response); err != nil {
			return err
		}
	}
}
