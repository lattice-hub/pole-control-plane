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

package logger

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pole-io/pole-server/apis"
	"github.com/pole-io/pole-server/apis/pkg/types"
	commonLog "github.com/pole-io/pole-server/pkg/common/log"
	"github.com/pole-io/pole-server/pkg/common/utils"
)

// 把操作记录记录到数据库中
const (
	// PluginName plugin name
	PluginName = "HistoryDB"
)

var log = commonLog.RegisterScope(PluginName, "", 0)

// init 初始化注册函数
// HistoryDB 历史记录logger
type HistoryDB struct {
	db *sql.DB
}

// Name 返回插件名字
func (h *HistoryDB) Name() string {
	return PluginName
}

// Destroy 销毁插件
func (h *HistoryDB) Destroy() error {
	return log.Sync()
}

// Initialize 插件初始化
func (h *HistoryDB) Initialize(c *apis.ConfigEntry) error {
	dsn := c.Option["dns"].(string)
	if dsn == "" {
		return errors.New("dns is required for HistoryDB plugin")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Errorf("failed to connect to database: %v", err)
		return err
	}
	h.db = db

	// Set connection parameters
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Initialize database and create table with partitioning if needed
	if err := db.Ping(); err != nil {
		log.Errorf("failed to ping database: %v", err)
		return err
	}
	return h.initDatabase(db)
}

func (h *HistoryDB) initDatabase(db *sql.DB) error {
	// Check if operation_history table exists
	var tableExists bool
	if err := db.QueryRow("SELECT 1 FROM information_schema.tables WHERE table_schema = DATABASE() AND table_name = 'operation_history'").Scan(&tableExists); err != nil && err != sql.ErrNoRows {
		log.Errorf("failed to check if table exists: %v", err)
		return err
	}
	// Create table with partitioning if it doesn't exist
	if tableExists {
		return nil
	}
	_, err := db.Exec(`
CREATE TABLE operation_history (
    id BIGINT NOT NULL AUTO_INCREMENT,
    resource_type VARCHAR(64) NOT NULL,
    namespace VARCHAR(128) NOT NULL,
    resource_name VARCHAR(128) NOT NULL,
    operation_type VARCHAR(64) NOT NULL,
    operator VARCHAR(64) NOT NULL,
    detail TEXT,
    happen_time DATETIME NOT NULL,
    server VARCHAR(128) NOT NULL,
    PRIMARY KEY (id, happen_time),
    KEY idx_happen_time (happen_time),
	KEY idx_resource_name (resource_type, resource_name),
    KEY idx_operator (operator)
)
PARTITION BY RANGE (YEAR(happen_time)*100 + MONTH(happen_time)) (
    PARTITION p202506 VALUES LESS THAN (202507),
    PARTITION p202507 VALUES LESS THAN (202508),
    PARTITION p202508 VALUES LESS THAN (202509),
    PARTITION p202509 VALUES LESS THAN (202510),
    PARTITION pmax VALUES LESS THAN MAXVALUE
);
		`)
	if err != nil {
		log.Errorf("failed to create operation_history table: %v", err)
		return err
	}

	// Add partitions for当前月和下月（如有需要可扩展）
	if err := h.updateMonthlyPartitions(); err != nil {
		log.Errorf("failed to initialize partitions: %v", err)
		return err
	}
	// 启动定时任务自动维护分区
	h.StartPartitionUpdater()
	return nil
}

func (h *HistoryDB) Type() apis.PluginType {
	return apis.PluginTypeHistory
}

// Record 记录操作记录到日志中
func (h *HistoryDB) Record(entry *types.RecordEntry) {
	entry.Server = utils.LocalHost
	// Ensure database connection is established
	if h.db == nil {
		log.Error("database connection not established, falling back to logging")
		return
	}

	// Prepare the SQL statement to insert the record
	_, err := h.db.Exec(`
		INSERT INTO operation_history 
		(resource_type, namespace, resource_name, operation_type, operator, detail, happen_time, server) 
		VALUES
		(?, ?, ?, ?, ?, ?, ?, ?)
	`, entry.ResourceType,
		entry.Namespace,
		entry.ResourceName,
		entry.OperationType,
		entry.Operator,
		types.SanitizeOperationDetail(entry.ResourceType, entry.Detail),
		entry.HappenTime,
		entry.Server)
	if err != nil {
		log.Errorf("failed to insert history record: %v", err)
		return
	}
}

// updateMonthlyPartitions 自动维护 operation_history 按月分区
func (h *HistoryDB) updateMonthlyPartitions() error {
	if h.db == nil {
		return errors.New("db is nil")
	}

	now := time.Now()
	// 维护当前月、下月、下下月分区，防止写入失败
	months := []time.Time{
		now,
		now.AddDate(0, 1, 0),
		now.AddDate(0, 2, 0),
	}

	for _, t := range months {
		partitionValue := t.Year()*100 + int(t.Month()) + 1
		partitionName := fmt.Sprintf("p%04d%02d", t.Year(), t.Month())
		// 检查分区是否存在
		var exists int
		query := `SELECT COUNT(1) FROM information_schema.PARTITIONS WHERE TABLE_SCHEMA=DATABASE() AND TABLE_NAME='operation_history' AND PARTITION_NAME=?`
		err := h.db.QueryRow(query, partitionName).Scan(&exists)
		if err != nil {
			return err
		}
		if exists == 0 {
			// 添加分区
			alter := fmt.Sprintf(`ALTER TABLE operation_history ADD PARTITION (PARTITION %s VALUES LESS THAN (%d))`, partitionName, partitionValue)
			if _, err := h.db.Exec(alter); err != nil {
				log.Errorf("failed to add partition %s: %v", partitionName, err)
				return err
			}
			log.Infof("add partition %s for operation_history", partitionName)
		}
	}
	return nil
}

// StartPartitionUpdater 启动定时分区维护任务，每天凌晨自动维护分区
func (h *HistoryDB) StartPartitionUpdater() {
	go func() {
		for {
			now := time.Now()
			next := now.AddDate(0, 0, 1)
			nextMidnight := time.Date(next.Year(), next.Month(), next.Day(), 0, 0, 0, 0, next.Location())
			d := nextMidnight.Sub(now)
			timer := time.NewTimer(d)
			<-timer.C
			if err := h.updateMonthlyPartitions(); err != nil {
				log.Errorf("auto update partitions failed: %v", err)
			} else {
				log.Infof("auto update partitions success")
			}
			timer.Stop()
		}
	}()
}
