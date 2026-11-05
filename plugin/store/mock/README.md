# 对Store的API的mock

## mock文件生成方法

在`./store`目录执行

```
mockgen -source=../../apis/store/api.go -aux_files github.com/pole-io/pole-server/apis/store=../../apis/store/config_inventory.go,github.com/pole-io/pole-server/apis/store=../../apis/store/discover_api.go,github.com/pole-io/pole-server/apis/store=../../apis/store/auth_api.go,github.com/pole-io/pole-server/apis/store=../../apis/store/admin_api.go,github.com/pole-io/pole-server/apis/store=../../apis/store/governance_api.go,github.com/pole-io/pole-server/apis/store=../../apis/store/ai.go -destination=mock/api_mock.go -package=mock

mockgen -source=mysql/admin.go -destination=mock/admin_mock.go -package=mock
```
