package defaultuser

import (
	"context"
	"testing"

	authtypes "github.com/pole-io/pole-server/apis/pkg/types/auth"
	apisecurity "github.com/pole-io/specification/source/go/api/v1/security"
)

func TestOrdinaryGroupUpdatePreservesTokenState(t *testing.T) {
	svr := &Server{authOpt: &AuthConfig{Salt: "0123456789abcdef"}}
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "disabled", true: "enabled"}[enabled], func(t *testing.T) {
			saved := &authtypes.UserGroupDetail{
				UserGroup: &authtypes.UserGroup{Comment: "before", TokenEnable: enabled},
				UserIds:   map[string]struct{}{"old": {}},
			}
			incoming, err := svr.createGroupModel(&apisecurity.UserGroup{
				Comment:  "after",
				Relation: &apisecurity.UserGroupRelation{Users: []*apisecurity.User{{Id: "new"}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			updated, changed := updateGroupFromModel(context.Background(), saved, incoming)
			if !changed || updated.Comment != "after" {
				t.Fatal("ordinary update did not apply comment")
			}
			if _, ok := updated.UserIds["new"]; !ok || len(updated.UserIds) != 1 {
				t.Fatal("ordinary update did not replace members")
			}
			if updated.TokenEnable != enabled {
				t.Fatalf("token state changed: got %v, want %v", updated.TokenEnable, enabled)
			}
		})
	}
}

func TestUpdateGroupAttributeUpdatesCommentForAuthorizedNonOwner(t *testing.T) {
	saved := &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{Comment: "before", Metadata: map[string]string{}},
		UserIds:   map[string]struct{}{},
	}
	incoming := &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{Comment: "after", Metadata: map[string]string{}},
		UserIds:   map[string]struct{}{},
	}

	updated, changed := updateGroupAttribute(context.Background(), saved, incoming)
	if !changed {
		t.Fatal("expected comment change to require persistence")
	}
	if updated.Comment != "after" {
		t.Fatalf("expected updated comment %q, got %q", "after", updated.Comment)
	}
}

func TestUpdateGroupAttributeCanClearComment(t *testing.T) {
	saved := &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{Comment: "before", Metadata: map[string]string{}},
		UserIds:   map[string]struct{}{},
	}
	incoming := &authtypes.UserGroupDetail{
		UserGroup: &authtypes.UserGroup{Comment: "", Metadata: map[string]string{}},
		UserIds:   map[string]struct{}{},
	}

	updated, changed := updateGroupAttribute(context.Background(), saved, incoming)
	if !changed {
		t.Fatal("expected clearing comment to require persistence")
	}
	if updated.Comment != "" {
		t.Fatalf("expected comment to be cleared, got %q", updated.Comment)
	}
}
