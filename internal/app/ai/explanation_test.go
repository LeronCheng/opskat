package ai

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/opskat/opskat/internal/repository/ai_provider_repo"
	"github.com/opskat/opskat/internal/repository/ai_provider_repo/mock_ai_provider_repo"
	"github.com/opskat/opskat/internal/service/command_explanation_svc"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestExplainCommandRejectsInvalidInputBeforeProviderAccess(t *testing.T) {
	old := ai_provider_repo.AIProvider()
	t.Cleanup(func() { ai_provider_repo.RegisterAIProvider(old) })
	ai_provider_repo.RegisterAIProvider(mock_ai_provider_repo.NewMockAIProviderRepo(gomock.NewController(t)))
	a := &AI{ctx: context.Background(), lang: providerTestLang{}}
	for _, command := range []command_explanation_svc.Command{
		{Command: " "}, {Command: strings.Repeat("a", 32769)},
		{Command: "pwd", Detail: strings.Repeat("a", 32769)},
		{Command: "pwd", Type: strings.Repeat("a", 65)},
		{Command: "pwd", AssetName: strings.Repeat("a", 256)},
	} {
		err := a.ExplainCommand(uuid.NewString(), command)
		require.ErrorContains(t, err, "invalid command explanation input")
	}
}

func TestCancelCommandExplanationCancelsOnlyItsRequest(t *testing.T) {
	first, cancelFirst := context.WithCancel(context.Background())
	defer cancelFirst()
	second, cancelSecond := context.WithCancel(context.Background())
	defer cancelSecond()
	id := uuid.NewString()
	a := &AI{currentConversationID: 41}
	a.explanationCancels.Store(id, cancelFirst)
	a.explanationCancels.Store(uuid.NewString(), cancelSecond)
	a.runners.Store(int64(41), "original runner")
	require.NoError(t, a.CancelCommandExplanation(id))
	require.ErrorIs(t, first.Err(), context.Canceled)
	require.NoError(t, second.Err())
	require.EqualValues(t, 41, a.currentConversationID)
	original, active := a.runners.Load(int64(41))
	require.True(t, active)
	require.Equal(t, "original runner", original)
	require.NoError(t, a.CancelCommandExplanation(uuid.NewString()), "already finished requests are harmless")
	require.Error(t, a.CancelCommandExplanation("invalid"))
}
