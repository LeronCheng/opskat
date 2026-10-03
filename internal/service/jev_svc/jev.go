package jev_svc

import (
	"context"
	"fmt"
	"strings"

	"github.com/cago-frame/cago/pkg/logger"
	"github.com/opskat/opskat/internal/bootstrap"
	"github.com/opskat/opskat/internal/model/entity/asset_entity"
	"github.com/opskat/opskat/internal/pkg/jev"
	"github.com/opskat/opskat/internal/service/credential_svc"
	"go.uber.org/zap"
)

func APIKey(ctx context.Context) (string, error) {
	logger.Ctx(ctx).Debug("load Jev API key started")
	cfg := bootstrap.GetConfig()
	if cfg.JevAPIKey == "" {
		logger.Ctx(ctx).Debug("load Jev API key completed", zap.Bool("configured", false))
		return "", nil
	}
	key, err := credential_svc.Default().Decrypt(cfg.JevAPIKey)
	if err != nil {
		logger.Ctx(ctx).Error("load Jev API key failed", zap.Error(err))
		return "", err
	}
	logger.Ctx(ctx).Debug("load Jev API key completed", zap.Bool("configured", true))
	return key, nil
}

func SaveAPIKey(ctx context.Context, key string) error {
	key = strings.TrimSpace(key)
	for _, ch := range key {
		if ch < 33 || ch > 126 {
			return fmt.Errorf("invalid Jev API key")
		}
	}
	if len(key) > 512 {
		return fmt.Errorf("jev API key exceeds 512 characters")
	}
	logger.Ctx(ctx).Info("save Jev configuration started")
	updatedConfig := *bootstrap.GetConfig()
	updatedConfig.JevAPIKey = ""
	if key != "" {
		encrypted, err := credential_svc.Default().Encrypt(key)
		if err != nil {
			logger.Ctx(ctx).Error("save Jev configuration failed", zap.Error(err))
			return err
		}
		updatedConfig.JevAPIKey = encrypted
	}
	if err := bootstrap.SaveConfig(&updatedConfig); err != nil {
		logger.Ctx(ctx).Error("save Jev configuration failed", zap.Error(err))
		return err
	}
	logger.Ctx(ctx).Info("save Jev configuration completed", zap.Bool("configured", key != ""))
	return nil
}

func Classify(ctx context.Context, asset *asset_entity.Asset, command string) (*jev.Classification, error) {
	key, err := APIKey(ctx)
	if err != nil {
		return jev.Unclassified("ERROR", "Jev API key decryption failed"), err
	}
	if key == "" {
		return jev.Unclassified("UNCONFIGURED", "Jev API key is not configured"), nil
	}
	cfg, err := asset.GetSSHConfig()
	if err != nil {
		return jev.Unclassified("ERROR", "invalid SSH configuration"), err
	}
	logger.Ctx(ctx).Info("Jev command classification started", zap.Int64("assetID", asset.ID), zap.String("model", jev.Model))
	result, err := jev.New(key).Classify(ctx, jev.State{Command: command, Context: map[string]any{
		"asset_type": asset.Type, "host": cfg.Host, "port": cfg.Port, "username": cfg.Username,
		"execution": "SSH exec request; remote default shell; environment, scripts and file targets are not verified",
		"output":    "returned through the OpsKat command result channel",
	}})
	if err != nil {
		result.Reason = err.Error()
		logger.Ctx(ctx).Warn("Jev command classification failed", zap.Int64("assetID", asset.ID), zap.Error(err))
		return result, err
	}
	logger.Ctx(ctx).Info("Jev command classification completed", zap.Int64("assetID", asset.ID), zap.String("level1", result.Level1), zap.String("status", result.Status), zap.Strings("level2", result.Level2))
	return result, nil
}
