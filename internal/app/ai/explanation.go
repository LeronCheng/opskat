package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/cago-frame/cago/pkg/logger"
	"github.com/google/uuid"
	"github.com/opskat/opskat/internal/app/i18n"
	"github.com/opskat/opskat/internal/service/command_explanation_svc"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"go.uber.org/zap"
)

type commandExplanationEvent struct {
	Content  string `json:"content"`
	Sequence uint64 `json:"sequence"`
	Error    string `json:"error,omitempty"`
	Done     bool   `json:"done,omitempty"`
}

// ExplainCommand acknowledges registration before starting the independent stream.
func (a *AI) ExplainCommand(requestID string, command command_explanation_svc.Command) error {
	if _, err := uuid.Parse(requestID); err != nil || len(requestID) != 36 {
		return fmt.Errorf("invalid command explanation request ID")
	}
	if strings.TrimSpace(command.Command) == "" || len(command.Command) > 32768 || len(command.Detail) > 32768 || len(command.Type) > 64 || len(command.AssetName) > 255 {
		return fmt.Errorf("invalid command explanation input")
	}
	ctx, cancel := context.WithCancel(i18n.Ctx(a.ctx, a.lang.Lang()))
	if _, loaded := a.explanationCancels.LoadOrStore(requestID, cancel); loaded {
		cancel()
		return fmt.Errorf("command explanation request already active")
	}
	ctx = logger.WithContextField(ctx, zap.String("explanationID", requestID))
	var content string
	var sequence uint64
	emit := func(event commandExplanationEvent) {
		sequence++
		event.Sequence = sequence
		event.Content = content
		runtime.EventsEmit(a.ctx, "ai:command-explanation:"+requestID, event)
	}
	go func() {
		defer a.explanationCancels.Delete(requestID)
		defer cancel()
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Ctx(ctx).Error("command explanation panicked", zap.Any("panic", recovered), zap.Stack("stack"))
				emit(commandExplanationEvent{Done: true, Error: "command explanation failed"})
			}
		}()
		_, err := command_explanation_svc.Explain(ctx, command, func(text string) {
			content = text
			emit(commandExplanationEvent{})
		})
		if err != nil {
			emit(commandExplanationEvent{Done: true, Error: err.Error()})
			return
		}
		emit(commandExplanationEvent{Done: true})
	}()
	return nil
}

func (a *AI) CancelCommandExplanation(requestID string) error {
	if _, err := uuid.Parse(requestID); err != nil || len(requestID) != 36 {
		return fmt.Errorf("invalid command explanation request ID")
	}
	if cancel, active := a.explanationCancels.Load(requestID); active {
		cancel.(context.CancelFunc)()
	}
	return nil
}
