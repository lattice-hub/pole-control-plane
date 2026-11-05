package sqldb

import (
	"database/sql"
	"errors"
	"time"

	storeapi "github.com/pole-io/pole-server/apis/store"
)

func ensureSystemConfigurationRuntimeSchema(db *BaseDB) error {
	statements := []string{
		`CREATE TABLE IF NOT EXISTS console_system_config_domain (
			component VARCHAR(64) NOT NULL,
			domain_name VARCHAR(64) NOT NULL,
			active_revision BIGINT NULL,
			draft_revision BIGINT NULL,
			mtime DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
			PRIMARY KEY (component, domain_name)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
		`CREATE TABLE IF NOT EXISTS console_system_config_revision (
			id BIGINT NOT NULL AUTO_INCREMENT,
			component VARCHAR(64) NOT NULL,
			domain_name VARCHAR(64) NOT NULL,
			payload_json JSON NOT NULL,
			secret_version_id BIGINT NOT NULL DEFAULT 0,
			state VARCHAR(16) NOT NULL,
			created_by VARCHAR(128) NOT NULL,
			published_by VARCHAR(128) NOT NULL DEFAULT '',
			ctime DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			published_at DATETIME(3) NULL,
			PRIMARY KEY (id),
			KEY idx_console_system_config_scope (component, domain_name, id)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
		`CREATE TABLE IF NOT EXISTS system_config_apply_receipt (
			component VARCHAR(64) NOT NULL,
			domain_name VARCHAR(64) NOT NULL,
			revision BIGINT NOT NULL,
			instance_id VARCHAR(192) NOT NULL,
			state VARCHAR(24) NOT NULL,
			message VARCHAR(1024) NOT NULL DEFAULT '',
			applied_at DATETIME(3) NULL,
			heartbeat_at DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
			PRIMARY KEY (component, domain_name, revision, instance_id),
			KEY idx_system_config_receipt_heartbeat (component, domain_name, heartbeat_at)
		) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin`,
	}
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}

func (s *stableStore) GetActiveSystemConfigRelease(component, domain string) (*storeapi.SystemConfigRelease, error) {
	release := &storeapi.SystemConfigRelease{}
	err := s.master.QueryRow(`SELECT r.id, r.payload_json
		FROM console_system_config_domain d
		JOIN console_system_config_revision r ON r.id = d.active_revision
		WHERE d.component = ? AND d.domain_name = ? AND r.state = 'published'`, component, domain).
		Scan(&release.Revision, &release.Payload)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return release, nil
}

func (s *stableStore) UpsertSystemConfigApplyReceipt(receipt storeapi.SystemConfigApplyReceipt) error {
	var appliedAt any
	if !receipt.AppliedAt.IsZero() {
		appliedAt = receipt.AppliedAt.UTC()
	}
	_, err := s.master.Exec(`INSERT INTO system_config_apply_receipt
		(component, domain_name, revision, instance_id, state, message, applied_at, heartbeat_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE state = VALUES(state), message = VALUES(message),
		applied_at = VALUES(applied_at), heartbeat_at = VALUES(heartbeat_at)`,
		receipt.Component, receipt.Domain, receipt.Revision, receipt.InstanceID,
		receipt.State, receipt.Message, appliedAt, time.Now().UTC())
	return err
}
