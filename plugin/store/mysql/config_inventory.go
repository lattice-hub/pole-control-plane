package sqldb

import "context"

// CountUnifiedConfigs 从主库统计环境内持久聚合和共享模板绑定；下线保留的历史仍阻止删除所属环境。
func (s *stableStore) CountUnifiedConfigs(ctx context.Context, environment string) (uint64, error) {
	var count uint64
	err := s.master.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT group_name, config_name FROM unified_config_state WHERE environment = ? UNION SELECT group_name, config_name FROM unified_config_file_environment WHERE environment = ?) files`, environment, environment).Scan(&count)
	return count, err
}

// CountUnifiedConfigsByEnvironment 返回主库内去重的配置文件数；空库返回空 map，查询失败返回错误。
func (s *stableStore) CountUnifiedConfigsByEnvironment(ctx context.Context) (map[string]uint64, error) {
	rows, err := s.master.DB.QueryContext(ctx, `SELECT environment, COUNT(*) FROM (SELECT environment, group_name, config_name FROM unified_config_state UNION SELECT environment, group_name, config_name FROM unified_config_file_environment) files GROUP BY environment`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	counts := make(map[string]uint64)
	for rows.Next() {
		var environment string
		var count uint64
		if err := rows.Scan(&environment, &count); err != nil {
			return nil, err
		}
		counts[environment] = count
	}
	return counts, rows.Err()
}
