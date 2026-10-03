package permission

import (
	"context"
	"fmt"
	"testing"

	"github.com/opskat/opskat/internal/ai/aictx"
	"github.com/opskat/opskat/internal/model/entity/asset_entity"
	"github.com/opskat/opskat/internal/pkg/jev"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func TestSSHAgentTrustSkipsConfirmationButHonorsBlacklist(t *testing.T) {
	ctx, repo, _ := setupPolicyTest(t)
	a := &asset_entity.Asset{ID: 1, Type: "ssh", Config: `{"host":"example","port":22,"username":"root","auth_type":"password","agent_operation_policy":"trust"}`}
	a.CmdPolicy = mustJSON(asset_entity.CommandPolicy{DenyList: []string{"rm -rf *"}})
	repo.EXPECT().Find(gomock.Any(), int64(1)).Return(a, nil).AnyTimes()
	assert.Equal(t, aictx.Allow, CheckPermission(ctx, "ssh", 1, "uname -a").Decision,
		"a trusted server must execute an ordinary command without prompting")
	assert.Equal(t, aictx.Deny, CheckPermission(ctx, "ssh", 1, "rm -rf /").Decision,
		"trust must never override the blacklist")
}

func TestSSHAgentPolicyControlsApprovalAndPreservesClassification(t *testing.T) {
	for _, mode := range []string{"", "approval", "safe_read", "read_only", "safe_write", "trust"} {
		for _, level := range []string{"SAFE_READ", "SENSITIVE_READ", "SAFE_CHANGE", "DANGEROUS_CHANGE", "UNKNOWN"} {
			t.Run(mode+"/"+level, func(t *testing.T) {
				ctx, repo, _ := setupPolicyTest(t)
				a := &asset_entity.Asset{ID: 1, Type: "ssh", Config: fmt.Sprintf(`{"agent_operation_policy":%q}`, mode)}
				repo.EXPECT().Find(gomock.Any(), int64(1)).Return(a, nil).AnyTimes()
				classification := jev.Unclassified("OK", "")
				classification.Level1 = level
				if level == "UNKNOWN" {
					classification.Status = "REVIEW"
				}
				ctx = WithCommandClassifier(ctx, func(context.Context, *asset_entity.Asset, string) (*jev.Classification, error) {
					return classification, nil
				})
				confirmed := 0
				checker := NewCommandPolicyChecker(func(context.Context, string, []ApprovalItem) ApprovalResponse {
					confirmed++
					return ApprovalResponse{Decision: "allow"}
				})
				result := checker.CheckForAsset(ctx, 1, "ssh", "uname -a")
				auto := mode == "trust" || (level == "SAFE_READ" && (mode == "safe_read" || mode == "read_only" || mode == "safe_write")) || (level == "SENSITIVE_READ" && (mode == "read_only" || mode == "safe_write")) || (level == "SAFE_CHANGE" && mode == "safe_write")
				if auto {
					assert.Zero(t, confirmed)
					assert.Equal(t, aictx.SourceAgentPolicy, result.DecisionSource)
				} else {
					assert.Equal(t, 1, confirmed)
					assert.Equal(t, aictx.SourceUserAllow, result.DecisionSource)
				}
				assert.Same(t, classification, result.Classification, "manual approval must retain the Jev audit evidence")
			})
		}
	}
}

func TestSSHAgentClassifierFailureCannotAutoApprove(t *testing.T) {
	ctx, repo, _ := setupPolicyTest(t)
	a := &asset_entity.Asset{ID: 1, Type: "ssh", Config: `{"agent_operation_policy":"safe_write"}`}
	repo.EXPECT().Find(gomock.Any(), int64(1)).Return(a, nil).AnyTimes()
	ctx = WithCommandClassifier(ctx, func(context.Context, *asset_entity.Asset, string) (*jev.Classification, error) {
		return &jev.Classification{Level1: "SAFE_READ", Status: "OK"}, fmt.Errorf("provider failed")
	})
	result := CheckPermission(ctx, "ssh", 1, "uname -a")
	assert.Equal(t, aictx.NeedConfirm, result.Decision)
	assert.Equal(t, "ERROR", result.Classification.Status)
}
