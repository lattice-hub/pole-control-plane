package agentworkbench

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"google.golang.org/protobuf/proto"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
	apimodel "github.com/pole-io/specification/source/go/api/v1/model"
)

const (
	ProposalPreviewReady      ProposalStatus = "preview_ready"
	ProposalApplying          ProposalStatus = "applying"
	ProposalWaitingForPublish ProposalStatus = "waiting_for_publish"
)

var (
	ErrInvalidRequest       = errors.New("invalid request")
	ErrNoChanges            = errors.New("no changes")
	ErrSensitiveResource    = errors.New("sensitive resource is not supported")
	ErrProposalNotFound     = errors.New("proposal not found")
	ErrProposalForbidden    = errors.New("proposal belongs to another user")
	ErrProposalExpired      = errors.New("proposal expired")
	ErrConfirmationMismatch = errors.New("confirmation does not match preview")
	ErrProposalApplying     = errors.New("proposal is already applying")
	ErrStalePreview         = errors.New("resource changed after preview")
	ErrProposalCapacity     = errors.New("agent proposal capacity reached")
)

type ProposalStatus string

type Actor struct {
	UserID    string
	Token     string
	RequestID string
}

type ResourceRef struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Group     string `json:"group"`
	Name      string `json:"name"`
}

type ConfigFile struct {
	ID            string                     `json:"id,omitempty"`
	Name          string                     `json:"name"`
	Namespace     string                     `json:"namespace"`
	Group         string                     `json:"group"`
	Content       string                     `json:"content"`
	Format        string                     `json:"format,omitempty"`
	Comment       string                     `json:"comment,omitempty"`
	Status        string                     `json:"status,omitempty"`
	Labels        map[string]string          `json:"labels,omitempty"`
	Mtime         string                     `json:"mtime,omitempty"`
	Encrypted     bool                       `json:"encrypted"`
	EncryptAlgo   string                     `json:"encryptAlgo,omitempty"`
	DraftRevision uint64                     `json:"draftRevision"`
	Definition    *configv1.ConfigDefinition `json:"-"`
}

type PrepareConfigFileRequest struct {
	Namespace      string  `json:"namespace"`
	Group          string  `json:"group"`
	Name           string  `json:"name"`
	DesiredContent string  `json:"desiredContent"`
	Comment        *string `json:"comment,omitempty"`
}

type Proposal struct {
	ID           string         `json:"id"`
	Version      uint64         `json:"version"`
	Status       ProposalStatus `json:"status"`
	Resource     ResourceRef    `json:"resource"`
	Before       ConfigFile     `json:"before"`
	After        ConfigFile     `json:"after"`
	BaselineHash string         `json:"baselineHash"`
	PreviewHash  string         `json:"previewHash"`
	ExpiresAt    time.Time      `json:"expiresAt"`
	Warnings     []string       `json:"warnings"`
	actorID      string
	idempotency  string
	receipt      *Receipt
}

type ConfirmRequest struct {
	ProposalID      string `json:"-"`
	ProposalVersion uint64 `json:"proposalVersion"`
	PreviewHash     string `json:"previewHash"`
	IdempotencyKey  string `json:"idempotencyKey"`
}

type Receipt struct {
	ProposalID string         `json:"proposalId"`
	Status     ProposalStatus `json:"status"`
	Resource   ResourceRef    `json:"resource"`
	RequestID  string         `json:"requestId,omitempty"`
	DetailURL  string         `json:"detailUrl"`
}

type ConfigFilePort interface {
	GetConfigFile(ctx context.Context, actor Actor, ref ResourceRef) (ConfigFile, string, error)
	UpdateConfigFile(ctx context.Context, actor Actor, file ConfigFile) (string, error)
}

type Options struct {
	TTL          time.Duration
	MaxProposals int
	Now          func() time.Time
	NewID        func() string
}

type Workbench struct {
	mu           sync.Mutex
	config       ConfigFilePort
	proposals    map[string]*Proposal
	ttl          atomic.Int64
	maxProposals int
	now          func() time.Time
	newID        func() string
}

func New(config ConfigFilePort, opts Options) *Workbench {
	if opts.TTL <= 0 {
		opts.TTL = 30 * time.Minute
	}
	if opts.MaxProposals <= 0 {
		opts.MaxProposals = 1024
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.NewID == nil {
		opts.NewID = randomID
	}
	workbench := &Workbench{
		config: config, proposals: make(map[string]*Proposal),
		maxProposals: opts.MaxProposals, now: opts.Now, newID: opts.NewID,
	}
	workbench.ttl.Store(int64(opts.TTL))
	return workbench
}

func (w *Workbench) SetTTL(ttl time.Duration) {
	if ttl > 0 {
		w.ttl.Store(int64(ttl))
	}
}

func (w *Workbench) SetUpstreamTimeout(timeout time.Duration) {
	if configurable, ok := w.config.(interface{ SetTimeout(time.Duration) }); ok {
		configurable.SetTimeout(timeout)
	}
}

func (w *Workbench) PrepareConfigFile(ctx context.Context, actor Actor, req PrepareConfigFileRequest) (*Proposal, error) {
	ref := ResourceRef{Kind: "config.file", Namespace: strings.TrimSpace(req.Namespace), Group: strings.TrimSpace(req.Group), Name: strings.TrimSpace(req.Name)}
	if actor.UserID == "" || ref.Namespace == "" || ref.Group == "" || ref.Name == "" {
		return nil, ErrInvalidRequest
	}
	current, _, err := w.config.GetConfigFile(ctx, actor, ref)
	if err != nil {
		return nil, err
	}
	if current.Encrypted {
		return nil, ErrSensitiveResource
	}
	if !utf8.ValidString(current.Content) || !utf8.ValidString(req.DesiredContent) || strings.IndexByte(current.Content, 0) >= 0 || strings.IndexByte(req.DesiredContent, 0) >= 0 {
		return nil, ErrSensitiveResource
	}
	if req.Comment != nil && *req.Comment != current.Comment {
		return nil, ErrInvalidRequest
	}
	desired := cloneConfigFile(current)
	desired.Content = req.DesiredContent
	baselineHash, err := hashValue(current)
	if err != nil {
		return nil, err
	}
	desiredHash, err := hashValue(desired)
	if err != nil {
		return nil, err
	}
	if baselineHash == desiredHash {
		return nil, ErrNoChanges
	}
	proposal := &Proposal{
		ID: w.newID(), Version: 1, Status: ProposalPreviewReady, Resource: ref,
		Before: cloneConfigFile(current), After: cloneConfigFile(desired), BaselineHash: baselineHash,
		ExpiresAt: w.now().Add(time.Duration(w.ttl.Load())), actorID: actor.UserID,
		Warnings: []string{
			"确认后只保存配置草稿，不会创建发布版本，也不会影响当前已发布配置。",
			"确认时会重新读取并校验草稿；并发修改由草稿版本 CAS 拒绝，不会覆盖其他人的更新。",
		},
	}
	proposal.PreviewHash, err = hashValue(struct {
		Resource     ResourceRef `json:"resource"`
		BaselineHash string      `json:"baselineHash"`
		After        ConfigFile  `json:"after"`
	}{proposal.Resource, proposal.BaselineHash, proposal.After})
	if err != nil {
		return nil, err
	}
	w.mu.Lock()
	if !w.reserveProposalSlotLocked(w.now()) {
		w.mu.Unlock()
		return nil, ErrProposalCapacity
	}
	w.proposals[proposal.ID] = proposal
	w.mu.Unlock()
	return cloneProposal(proposal), nil
}

func (w *Workbench) Confirm(ctx context.Context, actor Actor, req ConfirmRequest) (*Receipt, error) {
	if actor.UserID == "" || req.ProposalID == "" || req.ProposalVersion == 0 || req.PreviewHash == "" || req.IdempotencyKey == "" {
		return nil, ErrInvalidRequest
	}
	w.mu.Lock()
	proposal, ok := w.proposals[req.ProposalID]
	if !ok {
		w.mu.Unlock()
		return nil, ErrProposalNotFound
	}
	if proposal.actorID != actor.UserID {
		w.mu.Unlock()
		return nil, ErrProposalForbidden
	}
	if proposal.Version != req.ProposalVersion || proposal.PreviewHash != req.PreviewHash {
		w.mu.Unlock()
		return nil, ErrConfirmationMismatch
	}
	if proposal.Status == ProposalWaitingForPublish && proposal.receipt != nil {
		if proposal.idempotency != req.IdempotencyKey {
			w.mu.Unlock()
			return nil, ErrConfirmationMismatch
		}
		receipt := *proposal.receipt
		w.mu.Unlock()
		return &receipt, nil
	}
	if !w.now().Before(proposal.ExpiresAt) {
		w.mu.Unlock()
		return nil, ErrProposalExpired
	}
	if proposal.Status == ProposalApplying {
		w.mu.Unlock()
		return nil, ErrProposalApplying
	}
	proposal.Status = ProposalApplying
	proposal.idempotency = req.IdempotencyKey
	working := cloneProposal(proposal)
	w.mu.Unlock()

	current, _, err := w.config.GetConfigFile(ctx, actor, working.Resource)
	if err != nil {
		w.resetAfterFailure(req.ProposalID)
		return nil, err
	}
	currentHash, err := hashValue(current)
	if err != nil {
		w.resetAfterFailure(req.ProposalID)
		return nil, err
	}
	if currentHash != working.BaselineHash {
		w.resetAfterFailure(req.ProposalID)
		return nil, ErrStalePreview
	}
	requestID, err := w.config.UpdateConfigFile(ctx, actor, working.After)
	if err != nil {
		w.resetAfterFailure(req.ProposalID)
		var upstream *UpstreamError
		if errors.As(err, &upstream) && upstream.Code == uint32(apimodel.Code_DataConflict) {
			return nil, ErrStalePreview
		}
		return nil, err
	}
	receipt := &Receipt{
		ProposalID: working.ID, Status: ProposalWaitingForPublish, Resource: working.Resource,
		RequestID: requestID,
		DetailURL: "/configuration?environment=" + url.QueryEscape(working.Resource.Namespace) + "&group=" + url.QueryEscape(working.Resource.Group) + "&name=" + url.QueryEscape(working.Resource.Name),
	}
	w.mu.Lock()
	stored := w.proposals[req.ProposalID]
	stored.Status = ProposalWaitingForPublish
	stored.receipt = receipt
	w.mu.Unlock()
	copyReceipt := *receipt
	return &copyReceipt, nil
}

func (w *Workbench) reserveProposalSlotLocked(now time.Time) bool {
	for id, proposal := range w.proposals {
		if proposal.Status != ProposalWaitingForPublish && !now.Before(proposal.ExpiresAt) {
			delete(w.proposals, id)
		}
	}
	for len(w.proposals) >= w.maxProposals {
		var oldestID string
		var oldestExpiry time.Time
		for id, proposal := range w.proposals {
			if proposal.Status == ProposalPreviewReady && (oldestID == "" || proposal.ExpiresAt.Before(oldestExpiry)) {
				oldestID, oldestExpiry = id, proposal.ExpiresAt
			}
		}
		if oldestID == "" {
			return false
		}
		delete(w.proposals, oldestID)
	}
	return true
}

func (w *Workbench) resetAfterFailure(id string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if proposal := w.proposals[id]; proposal != nil && proposal.Status == ProposalApplying {
		proposal.Status = ProposalPreviewReady
		proposal.idempotency = ""
	}
}

func cloneConfigFile(file ConfigFile) ConfigFile {
	clone := file
	if file.Definition != nil {
		clone.Definition = proto.Clone(file.Definition).(*configv1.ConfigDefinition)
	}
	if file.Labels != nil {
		clone.Labels = make(map[string]string, len(file.Labels))
		for key, value := range file.Labels {
			clone.Labels[key] = value
		}
	}
	return clone
}

func cloneProposal(proposal *Proposal) *Proposal {
	clone := *proposal
	clone.Before = cloneConfigFile(proposal.Before)
	clone.After = cloneConfigFile(proposal.After)
	clone.Warnings = append([]string(nil), proposal.Warnings...)
	if proposal.receipt != nil {
		receipt := *proposal.receipt
		clone.receipt = &receipt
	}
	return &clone
}

func hashValue(value any) (string, error) {
	content, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(content)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

func randomID() string {
	var content [16]byte
	if _, err := rand.Read(content[:]); err != nil {
		return fmt.Sprintf("proposal-%d", time.Now().UnixNano())
	}
	return "proposal-" + hex.EncodeToString(content[:])
}
