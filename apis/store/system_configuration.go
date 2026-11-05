package store

import "time"

// SystemConfigRelease is the immutable active payload consumed by Pole Server
// runtime appliers. Draft ownership remains in the Console repository.
type SystemConfigRelease struct {
	Revision int64
	Payload  []byte
}

// SystemConfigApplyReceipt records one process' last observation of a desired
// revision. Heartbeats make stale instances distinguishable from converged ones.
type SystemConfigApplyReceipt struct {
	Component  string
	Domain     string
	Revision   int64
	InstanceID string
	State      string
	Message    string
	AppliedAt  time.Time
}

// SystemConfigRuntimeStore is deliberately a narrow optional capability. It is
// not embedded into Store so third-party Store plugins remain source compatible.
type SystemConfigRuntimeStore interface {
	GetActiveSystemConfigRelease(component, domain string) (*SystemConfigRelease, error)
	UpsertSystemConfigApplyReceipt(receipt SystemConfigApplyReceipt) error
}
