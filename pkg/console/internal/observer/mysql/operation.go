package mysql

import (
	"strconv"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/pkg/console/internal/common/model"
)

// CREATE TABLE `operation_history` (
//   `id` bigint NOT NULL AUTO_INCREMENT,
//   `resource_type` varchar(64) COLLATE utf8mb4_bin NOT NULL,
//   `namespace` varchar(128) COLLATE utf8mb4_bin NOT NULL,
//   `resource_name` varchar(128) COLLATE utf8mb4_bin NOT NULL,
//   `operation_type` varchar(64) COLLATE utf8mb4_bin NOT NULL,
//   `operator` varchar(64) COLLATE utf8mb4_bin NOT NULL,
//   `detail` text COLLATE utf8mb4_bin,
//   `happen_time` datetime NOT NULL,
//   `server` varchar(128) COLLATE utf8mb4_bin NOT NULL,
//   PRIMARY KEY (`id`,`happen_time`),
//   KEY `idx_happen_time` (`happen_time`),
//   KEY `idx_resource_name` (`resource_type`,`resource_name`),
//   KEY `idx_operator` (`operator`)
// ) ENGINE=InnoDB AUTO_INCREMENT=5 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin
// /*!50100 PARTITION BY RANGE (((year(`happen_time`) * 100) + month(`happen_time`)))
// (PARTITION p202506 VALUES LESS THAN (202507) ENGINE = InnoDB,
//  PARTITION p202507 VALUES LESS THAN (202508) ENGINE = InnoDB,
//  PARTITION p202508 VALUES LESS THAN (202509) ENGINE = InnoDB,
//  PARTITION p202509 VALUES LESS THAN (202510) ENGINE = InnoDB,
//  PARTITION pmax VALUES LESS THAN MAXVALUE ENGINE = InnoDB) */

type OperationFetcher struct {
	master *BaseDB
	slave  *BaseDB
}

// GetHistory 获取操作历史记录
// filter 参数可以包含以下键：
// - "namespace": 命名空间
// - "resource_type": 资源类型
// - "resource_name": 资源名称
// - "operation_type": 操作类型
// - "operator": 操作者
// - "start_time": 开始时间
// - "end_time": 结束时间
// - "limit": 限制返回的记录数
// - "cursor": 游标，用于分页
// - "direction": 翻页方向，"prev" 表示向前翻页，"next" 表示向后翻页
// 返回值是一个 OperationRecord 的切片，包含操作历史记录
func (o *OperationFetcher) GetHistory(filter map[string]string) ([]*model.OperationRecord, error) {
	query := "SELECT id, resource_type, namespace, resource_name, operation_type, operator, detail, happen_time, server FROM operation_history WHERE 1=1"
	var args []interface{}

	if ns, ok := filter["namespace"]; ok && ns != "" {
		query += " AND namespace LIKE ?"
		args = append(args, "%"+ns+"%")
	}
	if rtype, ok := filter["resource_type"]; ok && rtype != "" {
		query += " AND resource_type = ?"
		args = append(args, rtype)
	}
	if rname, ok := filter["resource_name"]; ok && rname != "" {
		query += " AND resource_name LIKE ?"
		args = append(args, "%"+rname+"%")
	}
	if otype, ok := filter["operation_type"]; ok && otype != "" {
		query += " AND operation_type = ?"
		args = append(args, otype)
	}
	if operator, ok := filter["operator"]; ok && operator != "" {
		query += " AND operator = ?"
		args = append(args, operator)
	}
	if startTime, ok := filter["start_time"]; ok && startTime != "" {
		query += " AND happen_time >= ?"
		args = append(args, startTime)
	}
	if endTime, ok := filter["end_time"]; ok && endTime != "" {
		query += " AND happen_time <= ?"
		args = append(args, endTime)
	}
	query += " ORDER BY id DESC"

	if limitStr, ok := filter["limit"]; ok && limitStr != "" {
		query += " LIMIT ? OFFSET ?"
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, err
		}
		args = append(args, limit)
		offset, err := strconv.Atoi(filter["offset"])
		if err != nil {
			return nil, err
		}
		args = append(args, offset)
	} else {
		query += " LIMIT 10 OFFSET 0"
	}

	rows, err := o.slave.Query(query, args...)
	if err != nil {
		if isMissingTableError(err) {
			return []*model.OperationRecord{}, nil
		}
		return nil, err
	}
	defer rows.Close()

	var records []*model.OperationRecord

	for rows.Next() {
		var id int64
		record := &model.OperationRecord{}
		if err := rows.Scan(
			&id,
			&record.ResourceType,
			&record.Namespace,
			&record.ResourceName,
			&record.OperationType,
			&record.Operator,
			&record.OperationDetail,
			&record.HappenTime,
			&record.Server,
		); err != nil {
			return nil, err
		}
		resource := types.Resource(record.ResourceType)
		record.OperationDetail = types.SanitizeOperationDetail(resource, record.OperationDetail)
		records = append(records, record)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}
