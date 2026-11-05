package otel

import (
	"context"
	"fmt"
	"time"

	"github.com/pole-io/pole-server/apis"
	"github.com/pole-io/pole-server/apis/pkg/types"
	commonlog "github.com/pole-io/pole-server/pkg/common/log"
	commonotel "github.com/pole-io/pole-server/pkg/common/otel"
	"github.com/pole-io/pole-server/pkg/common/utils"
)

const (
	PluginName = "otel"

	eventNameAuditOperation = "pole.audit.operation"
)

var log = commonlog.RegisterScope("HistoryOtel", "", 0)

type HistoryWorker struct {
	exporter *commonotel.LogExporter
}

func (h *HistoryWorker) Name() string {
	return PluginName
}

func (h *HistoryWorker) Initialize(conf *apis.ConfigEntry) error {
	config, err := commonotel.ConfigFromOptions(confOption(conf))
	if err != nil {
		log.Errorf("load otel history config failed, disable otel history exporter: %v", err)
		return nil
	}
	exporter, err := commonotel.NewLogExporter(&config)
	if err != nil {
		log.Errorf("initialize otel history exporter failed, disable otel history exporter: %v", err)
		return nil
	}
	h.exporter = exporter
	return nil
}

func (h *HistoryWorker) Type() apis.PluginType {
	return apis.PluginTypeHistory
}

func (h *HistoryWorker) Destroy() error {
	if h.exporter == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return h.exporter.Shutdown(ctx)
}

func (h *HistoryWorker) Record(entry *types.RecordEntry) {
	if h.exporter == nil || entry == nil {
		return
	}
	server := entry.Server
	if server == "" {
		server = utils.LocalHost
		entry.Server = server
	}
	happenTime := entry.HappenTime
	if happenTime.IsZero() {
		happenTime = time.Now()
	}
	h.exporter.Emit(commonotel.LogRecord{
		Timestamp:    happenTime,
		SeverityText: "INFO",
		Body:         operationBody(entry),
		Attributes: map[string]string{
			"event.name":                eventNameAuditOperation,
			"pole.event.kind":           "audit",
			"pole.audit.operation":      string(entry.OperationType),
			"pole.audit.operator":       entry.Operator,
			"pole.audit.result":         "success",
			"pole.namespace":            entry.Namespace,
			"pole.resource.type":        string(entry.ResourceType),
			"pole.resource.name":        entry.ResourceName,
			"pole.server.address":       server,
			"pole.control_plane.signal": "history",
		},
	})
}

func operationBody(entry *types.RecordEntry) string {
	if entry.Detail != "" {
		return types.SanitizeOperationDetail(entry.ResourceType, entry.Detail)
	}
	return fmt.Sprintf("%s %s/%s by %s", entry.OperationType, entry.ResourceType, entry.ResourceName, entry.Operator)
}

func confOption(conf *apis.ConfigEntry) map[string]interface{} {
	if conf == nil || conf.Option == nil {
		return map[string]interface{}{}
	}
	return conf.Option
}
