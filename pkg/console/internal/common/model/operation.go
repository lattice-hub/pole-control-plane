package model

var (
	_searchOperationLogParams = map[string]struct{}{
		"namespace":        {},
		"resource_type":    {},
		"resource_name":    {},
		"operation_type":   {},
		"operator":         {},
		"operation_detail": {},
		"start_time":       {},
		"end_time":         {},
		// prev表示向前翻页，next表示向后翻页
		"direction": {},
		"limit":     {},
		"cursor":    {},
	}
)

type OperationLogResponse struct {
	Code    uint32             `json:"code"`
	Info    string             `json:"info"`
	Total   uint64             `json:"total"`
	Size    uint32             `json:"size"`
	HasNext bool               `json:"has_next"`
	Data    []*OperationRecord `json:"data"`
	Cursor  string             `json:"cursor"`
}

type OperationRecord struct {
	Cursor          string `json:"cursor"`
	ResourceType    string `json:"resource_type"`
	ResourceName    string `json:"resource_name"`
	Namespace       string `json:"namespace"`
	OperationType   string `json:"operation_type"`
	Operator        string `json:"operator"`
	OperationDetail string `json:"operation_detail"`
	HappenTime      string `json:"happen_time"`
	Server          string `json:"server"` // 事件发生的服务器
}

// Resource 操作资源
type Resource string

// 定义包含的资源类型
const (
	RNamespace         Resource = "Namespace"
	RService           Resource = "Service"
	RRouting           Resource = "Routing"
	RCircuitBreaker    Resource = "CircuitBreaker"
	RInstance          Resource = "Instance"
	RRateLimit         Resource = "RateLimit"
	RUser              Resource = "User"
	RUserGroup         Resource = "UserGroup"
	RUserGroupRelation Resource = "UserGroupRelation"
	RAuthStrategy      Resource = "AuthStrategy"
)

// OperationType 操作类型
type OperationType string

// 定义包含的操作类型
const (
	// OCreate 新建
	OCreate OperationType = "Create"
	// ODelete 删除
	ODelete OperationType = "Delete"
	// OUpdate 更新
	OUpdate OperationType = "Update"
	// OUpdateIsolate 更新隔离状态
	OUpdateIsolate OperationType = "UpdateIsolate"
	// OUpdateToken 更新token
	OUpdateToken OperationType = "UpdateToken"
	// OUpdateGroup 更新用户-用户组关联关系
	OUpdateGroup OperationType = "UpdateGroup"
	// OEnableRateLimit 更新启用状态
	OUpdateEnable OperationType = "UpdateEnable"
)

var (
	ResourceTypeInfos = map[string]string{
		string(RNamespace):      "命名空间",
		string(RService):        "服务",
		string(RInstance):       "服务实例",
		string(RRouting):        "路由规则",
		string(RRateLimit):      "限流规则",
		string(RCircuitBreaker): "熔断规则",
		string(RUser):           "用户",
		string(RUserGroup):      "用户组",
		string(RAuthStrategy):   "鉴权策略",
	}

	OperationTypeInfos = map[string]string{
		string(OCreate):        "创建",
		string(ODelete):        "删除",
		string(OUpdate):        "更新",
		string(OUpdateIsolate): "更新实例隔离状态",
		string(OUpdateGroup):   "更新用户组",
		string(OUpdateEnable):  "更新启用状态",
		string(OUpdateToken):   "更新用户/用户组Token",
	}
)
