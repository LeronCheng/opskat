package permission

import (
	"context"

	"github.com/cago-frame/cago/pkg/logger"
	"github.com/opskat/opskat/internal/ai/aictx"
	"github.com/opskat/opskat/internal/ai/policy"
	"github.com/opskat/opskat/internal/model/entity/asset_entity"
	policyent "github.com/opskat/opskat/internal/model/entity/policy"
	"github.com/opskat/opskat/internal/pkg/jev"
	"github.com/opskat/opskat/internal/service/asset_svc"
	"github.com/opskat/opskat/internal/service/jev_svc"
	"go.uber.org/zap"
)

type CommandClassifier func(context.Context, *asset_entity.Asset, string) (*jev.Classification, error)
type classifierKey struct{}

// WithCommandClassifier supplies the classifier for an execution context.
func WithCommandClassifier(ctx context.Context, classifier CommandClassifier) context.Context {
	return context.WithValue(ctx, classifierKey{}, classifier)
}

func checkSSHAgentPermission(ctx context.Context, assetID int64, command string) aictx.CheckResult {
	asset, err := asset_svc.Asset().Get(ctx, assetID)
	if err != nil {
		return aictx.CheckResult{Decision: aictx.Deny, DecisionSource: aictx.SourcePolicyDeny, Message: err.Error()}
	}
	result := checkShellPolicy(ctx, asset, assetID, command)
	if result.Decision == aictx.Deny {
		result.Classification = jev.Unclassified("BLOCKED", "deterministic policy blocked the command before classification")
		return result
	}
	classifier, ok := ctx.Value(classifierKey{}).(CommandClassifier)
	if !ok {
		classifier = jev_svc.Classify
	}
	classification, classifyErr := classifier(ctx, asset, command)
	result.Classification = classification
	if classifyErr != nil {
		classification.Status = "ERROR"
		classification.Reason = classifyErr.Error()
	}
	// Explicit whitelist rules and remembered grants retain their existing authority.
	if result.Decision == aictx.Allow {
		return result
	}
	// Preserve the parser's diagnostic and approval contract before reading config.
	subcommands, parseErr := policy.ExtractSubCommands(command)
	if parseErr != nil || len(subcommands) == 0 {
		return result
	}
	cfg, err := asset.GetSSHConfig()
	if err != nil {
		result.Message = err.Error()
		return result
	}
	if err := cfg.AgentOperationPolicy.Validate(); err != nil {
		result.Message = err.Error()
		return result
	}
	// The shell parser cannot guarantee blacklist matching for an opaque input.
	// Keep its manual-review contract even on a trusted asset.
	if cfg.AgentOperationPolicy == policyent.AgentTrust || (classification.Status == "OK" && cfg.AgentOperationPolicy.Allows(classification.Level1)) {
		result.Decision = aictx.Allow
		result.DecisionSource = aictx.SourceAgentPolicy
		logger.Ctx(ctx).Info("agent operation policy allowed command", zap.Int64("assetID", assetID), zap.String("policy", string(cfg.AgentOperationPolicy)), zap.String("level1", classification.Level1))
	} else {
		logger.Ctx(ctx).Info("agent operation policy requires approval", zap.Int64("assetID", assetID), zap.String("policy", string(cfg.AgentOperationPolicy)), zap.String("level1", classification.Level1), zap.String("status", classification.Status))
	}
	return result
}
