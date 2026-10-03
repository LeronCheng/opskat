package command_explanation_svc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/opskat/opskat/internal/model/entity/ai_provider_entity"
	"github.com/opskat/opskat/internal/repository/ai_provider_repo"
	"github.com/opskat/opskat/internal/repository/ai_provider_repo/mock_ai_provider_repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func useProvider(t *testing.T, p *ai_provider_entity.AIProvider, err error) {
	t.Helper()
	old := ai_provider_repo.AIProvider()
	t.Cleanup(func() { ai_provider_repo.RegisterAIProvider(old) })
	repo := mock_ai_provider_repo.NewMockAIProviderRepo(gomock.NewController(t))
	ai_provider_repo.RegisterAIProvider(repo)
	repo.EXPECT().GetActive(gomock.Any()).Return(p, err).AnyTimes()
}

func TestExplainDoesNotInheritThinkingOrConversationState(t *testing.T) {
	for _, kind := range []string{"openai", "anthropic"} {
		t.Run(kind, func(t *testing.T) {
			requests := make(chan map[string]any, 2)
			sessions := make(chan string, 2)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request map[string]any
				if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
					t.Error(err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				requests <- request
				sessions <- r.Header.Get("X-Session")
				w.Header().Set("Content-Type", "text/event-stream")
				if kind == "openai" {
					_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"reasoning_content":"private thinking"}}]}`+"\n\n")
					_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"Effect. "}}]}`+"\n\n")
					_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":{"content":"Summary: done."},"finish_reason":"stop"}]}`+"\n\n"+"data: [DONE]\n\n")
				} else {
					_, _ = fmt.Fprint(w, `event: message_start
data: {"type":"message_start","message":{"id":"m1","type":"message","role":"assistant","model":"test-model","content":[],"usage":{"input_tokens":1,"output_tokens":0}}}

event: content_block_start
data: {"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":"","signature":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":"private thinking"}}

event: content_block_stop
data: {"type":"content_block_stop","index":0}

event: content_block_start
data: {"type":"content_block_start","index":1,"content_block":{"type":"text","text":""}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Effect. "}}

event: content_block_delta
data: {"type":"content_block_delta","index":1,"delta":{"type":"text_delta","text":"Summary: done."}}

event: content_block_stop
data: {"type":"content_block_stop","index":1}

event: message_delta
data: {"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":2}}

event: message_stop
data: {"type":"message_stop"}

`)
				}
			}))
			defer server.Close()
			p := &ai_provider_entity.AIProvider{Type: kind, APIBase: server.URL, Model: "test-model", ReasoningEnabled: true, ReasoningEffort: "high"}
			require.NoError(t, p.SetExtraHeaders([]ai_provider_entity.ExtraHeader{{Name: "X-Session", Value: "{{session}}"}}))
			useProvider(t, p, nil)
			command := Command{Type: "exec", Command: "cat app.log | tail -10", AssetName: "web-01"}
			var previousSession string
			for range 2 {
				var streamed string
				answer, err := Explain(context.Background(), command, func(text string) { streamed = text })
				require.NoError(t, err)
				assert.Equal(t, "Effect. Summary: done.", answer)
				assert.Equal(t, answer, streamed)
				request := <-requests
				assert.NotContains(t, request, "thinking")
				assert.NotContains(t, request, "reasoning_effort")
				assert.NotContains(t, request, "tools")
				assert.Equal(t, true, request["stream"])
				messages := request["messages"].([]any)
				if kind == "openai" {
					require.Len(t, messages, 2)
					assert.Equal(t, "system", messages[0].(map[string]any)["role"])
				} else {
					require.Len(t, messages, 1)
					assert.Contains(t, request, "system")
				}
				user := messages[len(messages)-1].(map[string]any)
				assert.Equal(t, "user", user["role"])
				data, err := json.Marshal(user["content"])
				require.NoError(t, err)
				assert.Contains(t, string(data), "cat app.log | tail -10")
				session := <-sessions
				assert.NotEmpty(t, session)
				assert.NotEqual(t, previousSession, session, "each explanation has its own external session")
				previousSession = session
			}
			assert.True(t, p.ReasoningEnabled)
			assert.Equal(t, "high", p.ReasoningEffort)
		})
	}
}

func TestExplainSurfacesProviderFailures(t *testing.T) {
	t.Run("unconfigured", func(t *testing.T) {
		useProvider(t, nil, nil)
		answer, err := Explain(context.Background(), Command{Command: "pwd"}, func(string) {})
		require.ErrorContains(t, err, "Configure an AI provider")
		assert.Empty(t, answer)
	})
	t.Run("lookup failure", func(t *testing.T) {
		useProvider(t, nil, errors.New("database unavailable"))
		_, err := Explain(context.Background(), Command{Command: "pwd"}, func(string) {})
		require.ErrorContains(t, err, "database unavailable")
	})
	for _, test := range []struct{ name, message, finish, want string }{
		{"truncated", `{"content":"incomplete"}`, "length", "truncated"},
		{"empty", `{"content":" "}`, "stop", "no command explanation"},
		{"tool call", `{"content":"answer","tool_calls":[{"id":"t1","type":"function","function":{"name":"exec","arguments":"{}"}}]}`, "tool_calls", "tool call"},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprint(w, `data: {"choices":[{"index":0,"delta":`+test.message+`,"finish_reason":"`+test.finish+`"}]}`+"\n\n"+"data: [DONE]\n\n")
			}))
			defer server.Close()
			useProvider(t, &ai_provider_entity.AIProvider{Type: "openai", APIBase: server.URL, Model: "test"}, nil)
			answer, err := Explain(context.Background(), Command{Command: "pwd"}, func(string) {})
			require.ErrorContains(t, err, test.want)
			assert.Empty(t, answer)
		})
	}
}

func TestExplainForwardsTextBeforeCompletionAndHonorsCancellation(t *testing.T) {
	for _, outcome := range []string{"complete", "cancel", "interrupted"} {
		t.Run(outcome, func(t *testing.T) {
			release := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"First \"}}]}\n\n")
				w.(http.Flusher).Flush()
				select {
				case <-r.Context().Done():
					return
				case <-release:
				}
				if outcome == "complete" {
					_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"answer. Summary: done.\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
				}
			}))
			defer server.Close()
			useProvider(t, &ai_provider_entity.AIProvider{Type: "openai", APIBase: server.URL, Model: "test"}, nil)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			text := make(chan string, 2)
			result := make(chan error, 1)
			go func() {
				_, err := Explain(ctx, Command{Command: "pwd"}, func(delta string) { text <- delta })
				result <- err
			}()
			select {
			case first := <-text:
				assert.Equal(t, "First ", first)
			case <-time.After(3 * time.Second):
				t.Fatal("first text was buffered until completion")
			}
			select {
			case <-result:
				t.Fatal("explanation finished before the provider completed")
			default:
			}
			if outcome == "cancel" {
				cancel()
			}
			close(release)
			select {
			case err := <-result:
				switch outcome {
				case "complete":
					require.NoError(t, err)
				case "cancel":
					require.ErrorIs(t, err, context.Canceled)
				default:
					require.Error(t, err, "an interrupted stream must not report a complete answer")
				}
			case <-time.After(3 * time.Second):
				t.Fatal("explanation did not finish or cancel")
			}
		})
	}
}
