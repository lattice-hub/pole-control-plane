package sqldb

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pole-io/pole-server/apis/pkg/types"
	"github.com/pole-io/pole-server/apis/store"
)

type environmentPromotionStore struct {
	master *BaseDB
}

func (s *environmentPromotionStore) GetEnvironmentPromotionTopology() (*types.EnvironmentPromotionTopology, error) {
	row := s.master.QueryRow(`SELECT draft_revision, published_revision, draft_json, modify_by, UNIX_TIMESTAMP(mtime)
		FROM environment_promotion_topology WHERE id = 1`)
	var topology types.EnvironmentPromotionTopology
	var payload, modifyBy string
	var modifyTime int64
	if err := row.Scan(&topology.DraftRevision, &topology.PublishedRevision, &payload,
		&modifyBy, &modifyTime); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &types.EnvironmentPromotionTopology{}, nil
		}
		return nil, store.Error(err)
	}
	if err := json.Unmarshal([]byte(payload), &topology); err != nil {
		return nil, store.Error(err)
	}
	topology.ModifyBy = modifyBy
	topology.ModifyTime = time.Unix(modifyTime, 0)
	return &topology, nil
}

func (s *environmentPromotionStore) SaveEnvironmentPromotionTopology(
	topology *types.EnvironmentPromotionTopology, expectedRevision uint64) error {
	tx, err := s.master.Begin()
	if err != nil {
		return store.Error(err)
	}
	defer func() { _ = tx.Rollback() }()

	var current uint64
	err = tx.QueryRow(`SELECT draft_revision FROM environment_promotion_topology WHERE id = 1 FOR UPDATE`).Scan(&current)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		if expectedRevision != 0 {
			return store.ErrEnvironmentPromotionRevisionConflict
		}
		topology.DraftRevision = 1
		payload, marshalErr := json.Marshal(topology)
		if marshalErr != nil {
			return store.Error(marshalErr)
		}
		_, err = tx.Exec(`INSERT INTO environment_promotion_topology
			(id, draft_revision, published_revision, draft_json, modify_by)
			VALUES (1, 1, 0, ?, ?)`, payload, topology.ModifyBy)
	case err != nil:
		return store.Error(err)
	case current != expectedRevision:
		return store.ErrEnvironmentPromotionRevisionConflict
	default:
		topology.DraftRevision = current + 1
		payload, marshalErr := json.Marshal(topology)
		if marshalErr != nil {
			return store.Error(marshalErr)
		}
		_, err = tx.Exec(`UPDATE environment_promotion_topology
			SET draft_revision = ?, draft_json = ?, modify_by = ?, mtime = sysdate()
			WHERE id = 1`, topology.DraftRevision, payload, topology.ModifyBy)
	}
	if err != nil {
		return store.Error(err)
	}
	return tx.Commit()
}

func (s *environmentPromotionStore) PublishEnvironmentPromotionTopology(
	expectedRevision uint64, createBy, comment string) (*types.EnvironmentPromotionTopologyRevision, error) {
	tx, err := s.master.Begin()
	if err != nil {
		return nil, store.Error(err)
	}
	defer func() { _ = tx.Rollback() }()

	var draftRevision, publishedRevision uint64
	var payload string
	if err := tx.QueryRow(`SELECT draft_revision, published_revision, draft_json
		FROM environment_promotion_topology WHERE id = 1 FOR UPDATE`).Scan(
		&draftRevision, &publishedRevision, &payload); err != nil {
		return nil, store.Error(err)
	}
	if draftRevision != expectedRevision {
		return nil, store.ErrEnvironmentPromotionRevisionConflict
	}
	var topology types.EnvironmentPromotionTopology
	if err := json.Unmarshal([]byte(payload), &topology); err != nil {
		return nil, store.Error(err)
	}
	revision := publishedRevision + 1
	topology.PublishedRevision = revision
	topology.DraftRevision = draftRevision
	payloadBytes, err := json.Marshal(&topology)
	if err != nil {
		return nil, store.Error(err)
	}
	if _, err := tx.Exec(`INSERT INTO environment_promotion_topology_revision
		(revision, topology_json, comment, create_by) VALUES (?, ?, ?, ?)`,
		revision, payloadBytes, comment, createBy); err != nil {
		return nil, store.Error(err)
	}
	if _, err := tx.Exec(`UPDATE environment_promotion_topology
		SET published_revision = ?, draft_json = ?, modify_by = ?, mtime = sysdate() WHERE id = 1`,
		revision, payloadBytes, createBy); err != nil {
		return nil, store.Error(err)
	}
	if err := tx.Commit(); err != nil {
		return nil, store.Error(err)
	}
	return &types.EnvironmentPromotionTopologyRevision{
		Revision: revision, Topology: topology, Comment: comment, CreateBy: createBy,
	}, nil
}

func (s *environmentPromotionStore) ListEnvironmentPromotionTopologyRevisions() (
	[]*types.EnvironmentPromotionTopologyRevision, error) {
	rows, err := s.master.Query(`SELECT revision, topology_json, comment, create_by, UNIX_TIMESTAMP(ctime)
		FROM environment_promotion_topology_revision ORDER BY revision DESC`)
	if err != nil {
		return nil, store.Error(err)
	}
	defer rows.Close()
	revisions := make([]*types.EnvironmentPromotionTopologyRevision, 0)
	for rows.Next() {
		var revision types.EnvironmentPromotionTopologyRevision
		var payload string
		var createTime int64
		if err := rows.Scan(&revision.Revision, &payload, &revision.Comment,
			&revision.CreateBy, &createTime); err != nil {
			return nil, store.Error(err)
		}
		revision.CreateTime = time.Unix(createTime, 0)
		if err := json.Unmarshal([]byte(payload), &revision.Topology); err != nil {
			return nil, store.Error(fmt.Errorf("decode topology revision %d: %w", revision.Revision, err))
		}
		revisions = append(revisions, &revision)
	}
	return revisions, store.Error(rows.Err())
}
