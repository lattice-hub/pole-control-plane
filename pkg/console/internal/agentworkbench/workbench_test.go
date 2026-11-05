package agentworkbench

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	configv1 "github.com/pole-io/specification/source/go/api/v1/config"
)

type fakeConfigFilePort struct {
	file        ConfigFile
	getErr      error
	updateErr   error
	updateCalls int
}

func (f *fakeConfigFilePort) GetConfigFile(_ context.Context, _ Actor, _ ResourceRef) (ConfigFile, string, error) {
	return cloneConfigFile(f.file), "read-request", f.getErr
}

func (f *fakeConfigFilePort) UpdateConfigFile(_ context.Context, _ Actor, file ConfigFile) (string, error) {
	f.updateCalls++
	if f.updateErr != nil {
		return "write-request", f.updateErr
	}
	f.file = cloneConfigFile(file)
	return "write-request", nil
}

func TestWorkbenchPrepareAndConfirmConfigFileStagesWithoutPublish(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	port := &fakeConfigFilePort{file: ConfigFile{
		ID: "10", Namespace: "default", Group: "orders", Name: "application.yaml",
		Content: "timeout: 3s\n", Format: "yaml", Comment: "current", Labels: map[string]string{"env": "test"},
	}}
	wb := New(port, Options{
		TTL:   time.Hour,
		Now:   func() time.Time { return now },
		NewID: func() string { return "proposal-1" },
	})
	actor := Actor{UserID: "alice", Token: "token", RequestID: "prepare-request"}

	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "orders", Name: "application.yaml",
		DesiredContent: "timeout: 5s\n",
	})
	require.NoError(t, err)
	require.Equal(t, ProposalPreviewReady, proposal.Status)
	require.Equal(t, "timeout: 3s\n", proposal.Before.Content)
	require.Equal(t, "timeout: 5s\n", proposal.After.Content)
	require.NotEmpty(t, proposal.PreviewHash)
	require.Equal(t, now.Add(time.Hour), proposal.ExpiresAt)
	require.Zero(t, port.updateCalls)

	receipt, err := wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.NoError(t, err)
	require.Equal(t, ProposalWaitingForPublish, receipt.Status)
	require.Equal(t, "write-request", receipt.RequestID)
	require.Equal(t, "/configuration?environment=default&group=orders&name=application.yaml", receipt.DetailURL)
	require.Equal(t, 1, port.updateCalls)
	require.Equal(t, "timeout: 5s\n", port.file.Content)
	require.Equal(t, "current", port.file.Comment)
}

func TestWorkbenchConfirmIsIdempotent(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	wb := New(port, Options{NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice", Token: "token"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)

	first, err := wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.NoError(t, err)
	second, err := wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, port.updateCalls)
}

func TestWorkbenchReplaysAppliedReceiptAfterProposalExpiry(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	wb := New(port, Options{TTL: time.Minute, Now: func() time.Time { return now }, NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)
	request := ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	}
	first, err := wb.Confirm(context.Background(), actor, request)
	require.NoError(t, err)
	now = now.Add(2 * time.Minute)
	replayed, err := wb.Confirm(context.Background(), actor, request)
	require.NoError(t, err)
	require.Equal(t, first, replayed)
	require.Equal(t, 1, port.updateCalls)
}

func TestWorkbenchRejectsMismatchedProposalVersion(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	wb := New(port, Options{NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)
	_, err = wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version + 1, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.ErrorIs(t, err, ErrConfirmationMismatch)
	require.Zero(t, port.updateCalls)
}

func TestWorkbenchBoundsInMemoryProposalContent(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	ids := []string{"proposal-1", "proposal-2", "proposal-3"}
	wb := New(port, Options{MaxProposals: 2, NewID: func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}})
	actor := Actor{UserID: "alice"}
	for i := 0; i < 3; i++ {
		_, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
			Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: string(rune('2' + i)),
		})
		require.NoError(t, err)
	}
	require.Len(t, wb.proposals, 2)
}

func TestWorkbenchPreservesAppliedReceiptWhenCapacityIsReached(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	ids := []string{"proposal-1", "proposal-2"}
	wb := New(port, Options{MaxProposals: 1, NewID: func() string {
		id := ids[0]
		ids = ids[1:]
		return id
	}})
	actor := Actor{UserID: "alice"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)
	confirm := ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	}
	receipt, err := wb.Confirm(context.Background(), actor, confirm)
	require.NoError(t, err)

	_, err = wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 3\n",
	})
	require.ErrorIs(t, err, ErrProposalCapacity)
	replayed, err := wb.Confirm(context.Background(), actor, confirm)
	require.NoError(t, err)
	require.Equal(t, receipt, replayed)
	require.Equal(t, 1, port.updateCalls)
}

func TestWorkbenchConfirmRejectsStalePreview(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	wb := New(port, Options{NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice", Token: "token"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)
	port.file.Content = "a: 9\n"

	_, err = wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.ErrorIs(t, err, ErrStalePreview)
	require.Zero(t, port.updateCalls)
}

func TestWorkbenchConfirmRejectsAdvancedDraftRevisionEvenWhenTextMatches(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n", DraftRevision: 3}}
	wb := New(port, Options{NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)
	port.file.DraftRevision = 4
	_, err = wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.ErrorIs(t, err, ErrStalePreview)
	require.Zero(t, port.updateCalls)
}

func TestWorkbenchConfirmRejectsExpiredOrForeignProposal(t *testing.T) {
	now := time.Date(2026, 7, 21, 10, 0, 0, 0, time.UTC)
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	wb := New(port, Options{
		TTL:   time.Minute,
		Now:   func() time.Time { return now },
		NewID: func() string { return "proposal-1" },
	})
	proposal, err := wb.PrepareConfigFile(context.Background(), Actor{UserID: "alice"}, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)

	_, err = wb.Confirm(context.Background(), Actor{UserID: "bob"}, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.ErrorIs(t, err, ErrProposalForbidden)

	now = now.Add(2 * time.Minute)
	_, err = wb.Confirm(context.Background(), Actor{UserID: "alice"}, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.ErrorIs(t, err, ErrProposalExpired)
}

func TestWorkbenchRejectsEncryptedAndNoopChanges(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{
		Namespace: "default", Group: "g", Name: "secret.yaml", Content: "secret: redacted\n", Encrypted: true,
	}}
	wb := New(port, Options{})
	_, err := wb.PrepareConfigFile(context.Background(), Actor{UserID: "alice"}, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "secret.yaml", DesiredContent: "secret: new\n",
	})
	require.ErrorIs(t, err, ErrSensitiveResource)

	port.file.Encrypted = false
	_, err = wb.PrepareConfigFile(context.Background(), Actor{UserID: "alice"}, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "secret.yaml", DesiredContent: port.file.Content,
	})
	require.ErrorIs(t, err, ErrNoChanges)
}

func TestWorkbenchRejectsUnsupportedCommentAndBinaryEdits(t *testing.T) {
	port := &fakeConfigFilePort{file: ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"}}
	wb := New(port, Options{})
	actor := Actor{UserID: "alice"}
	_, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n", Comment: stringPointer("not supported by v1 draft"),
	})
	require.ErrorIs(t, err, ErrInvalidRequest)
	_, err = wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: string([]byte{0xff}),
	})
	require.ErrorIs(t, err, ErrSensitiveResource)
	port.file.Content = string([]byte{0xff})
	_, err = wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.ErrorIs(t, err, ErrSensitiveResource)
}

func TestCloneConfigFilePreservesIndependentDefinition(t *testing.T) {
	original := ConfigFile{Definition: &configv1.ConfigDefinition{
		Content: []byte("old"), DefaultValues: map[string]*configv1.Scalar{"x": {Value: "one"}},
	}}
	copy := cloneConfigFile(original)
	copy.Definition.Content[0] = 'n'
	copy.Definition.DefaultValues["x"].Value = "two"
	require.Equal(t, "old", string(original.Definition.Content))
	require.Equal(t, "one", original.Definition.DefaultValues["x"].Value)
}

func TestWorkbenchPropagatesAdapterFailureWithoutApplying(t *testing.T) {
	port := &fakeConfigFilePort{
		file:      ConfigFile{Namespace: "default", Group: "g", Name: "a.yaml", Content: "a: 1\n"},
		updateErr: errors.New("upstream failed"),
	}
	wb := New(port, Options{NewID: func() string { return "proposal-1" }})
	actor := Actor{UserID: "alice"}
	proposal, err := wb.PrepareConfigFile(context.Background(), actor, PrepareConfigFileRequest{
		Namespace: "default", Group: "g", Name: "a.yaml", DesiredContent: "a: 2\n",
	})
	require.NoError(t, err)
	_, err = wb.Confirm(context.Background(), actor, ConfirmRequest{
		ProposalID: proposal.ID, ProposalVersion: proposal.Version, PreviewHash: proposal.PreviewHash, IdempotencyKey: "confirm-1",
	})
	require.EqualError(t, err, "upstream failed")
	require.Equal(t, 1, port.updateCalls)
}

func stringPointer(value string) *string { return &value }
