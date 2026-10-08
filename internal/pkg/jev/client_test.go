package jev

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func primaryAnswer(level string) map[string]any {
	probabilities := map[string]float64{}
	for key := range rules.Primary["level1"].Criteria {
		probabilities[key] = 0.0025
	}
	probabilities[level] = 0.99
	return map[string]any{"choice": level, "confidence": 0.99, "probabilities": probabilities}
}

func primaryAnswerWithProbability(level string, chosen, confidence float64) map[string]any {
	probabilities := map[string]float64{}
	other := (1 - chosen) / float64(len(rules.Primary["level1"].Criteria)-1)
	for key := range rules.Primary["level1"].Criteria {
		probabilities[key] = other
	}
	probabilities[level] = chosen
	return map[string]any{"choice": level, "confidence": confidence, "probabilities": probabilities}
}

func TestClassifyRoutesSecondaryQuestionsWithFullCommand(t *testing.T) {
	for _, level := range []string{"SAFE_READ", "SAFE_CHANGE", "UNKNOWN", "SENSITIVE_READ", "DANGEROUS_CHANGE"} {
		t.Run(level, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
				var request struct {
					Model     string
					State     State
					Questions map[string]question
				}
				require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
				require.Equal(t, Model, request.Model)
				require.Equal(t, "cat /etc/shadow; systemctl restart nginx", request.State.Command)
				require.Equal(t, "root", request.State.Context["username"])
				answers := map[string]any{}
				calls++
				if calls == 1 {
					require.Len(t, request.Questions, 1)
					answers["level1"] = primaryAnswer(level)
				} else {
					require.Equal(t, level, request.State.PriorLevel1)
					require.Equal(t, len(rules.Secondary[level]), len(request.Questions))
					for key, q := range request.Questions {
						require.Equal(t, "noul", q.Type)
						_, exists := rules.Secondary[level][key]
						require.True(t, exists, "only selected branch questions may be sent")
						answers[key] = map[string]float64{"noul": 0.99}
					}
				}
				require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"model": Model, "answers": answers}))
			}))
			defer server.Close()
			client := New("test-key")
			client.Endpoint = server.URL
			result, err := client.Classify(context.Background(), State{Command: "cat /etc/shadow; systemctl restart nginx", Context: map[string]any{"username": "root"}})
			require.NoError(t, err)
			require.Equal(t, level, result.Level1)
			if branch, ok := rules.Secondary[level]; ok {
				require.Equal(t, 2, calls)
				require.Len(t, result.Level2, 1)
				require.Contains(t, branch, result.Level2[0])
			} else {
				require.Equal(t, 1, calls)
				require.Empty(t, result.Level2)
			}
			if level == "UNKNOWN" {
				require.Equal(t, "REVIEW", result.Status)
			} else {
				require.Equal(t, "OK", result.Status)
			}
		})
	}
}

func TestClassifyUsesStrictlyGreaterPrimaryConfidenceThreshold(t *testing.T) {
	for _, test := range []struct {
		name      string
		chosen    float64
		threshold float64
		expectOK  bool
	}{
		{name: "exact threshold is review", chosen: 0.5, threshold: 0.5, expectOK: false},
		{name: "above threshold is accepted", chosen: 0.51, threshold: 0.5, expectOK: true},
		{name: "configured threshold", chosen: 0.76, threshold: 0.75, expectOK: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				answers := map[string]any{"level1": primaryAnswerWithProbability("SAFE_READ", test.chosen, 0.99)}
				_ = json.NewEncoder(w).Encode(map[string]any{"model": Model, "answers": answers})
			}))
			defer server.Close()

			client := New("test-key")
			client.Endpoint = server.URL
			client.PrimaryConfidenceThreshold = test.threshold
			result, err := client.Classify(context.Background(), State{Command: "cat /etc/hosts"})
			require.NoError(t, err)
			require.Equal(t, 1, calls)
			require.Equal(t, test.expectOK, result.Status == "OK")
		})
	}
}

func TestClassifyKeepsOnlyHighestSecondaryProbability(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			State State
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&request))
		answers := map[string]any{}
		if request.State.PriorLevel1 == "" {
			answers["level1"] = primaryAnswer("DANGEROUS_CHANGE")
		} else {
			for key := range rules.Secondary["DANGEROUS_CHANGE"] {
				p := 0.81
				if key == "SERVICE_HOST_CHANGE" {
					p = 0.97
				}
				answers[key] = map[string]float64{"noul": p}
			}
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"model": Model, "answers": answers}))
	}))
	defer server.Close()

	client := New("test-key")
	client.Endpoint = server.URL
	result, err := client.Classify(context.Background(), State{Command: "systemctl restart nginx"})
	require.NoError(t, err)
	require.Equal(t, []string{"SERVICE_HOST_CHANGE"}, result.Level2)
}

func TestClassifyDoesNotDowngradeUncertainOrMissingSecondary(t *testing.T) {
	for _, scenario := range []string{"uncertain", "missing", "http-error", "no-tags", "low-primary-confidence"} {
		t.Run(scenario, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				answers := map[string]any{}
				if calls == 1 {
					a := primaryAnswer("DANGEROUS_CHANGE")
					if scenario == "low-primary-confidence" {
						a["confidence"] = 0.5
					}
					answers["level1"] = a
				} else {
					if scenario == "http-error" {
						w.WriteHeader(http.StatusUnauthorized)
						return
					}
					for key := range rules.Secondary["DANGEROUS_CHANGE"] {
						p := 0.99
						if scenario == "uncertain" {
							p = 0.5
						}
						if scenario == "no-tags" {
							p = 0.01
						}
						answers[key] = map[string]float64{"noul": p}
					}
					if scenario == "missing" {
						delete(answers, "CONFIG_CHANGE")
					}
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"model": Model, "answers": answers})
			}))
			defer server.Close()
			client := New("test-key")
			client.Endpoint = server.URL
			result, err := client.Classify(context.Background(), State{Command: "systemctl restart nginx"})
			require.Equal(t, "DANGEROUS_CHANGE", result.Level1)
			require.Equal(t, 2, calls, "even a low-confidence branch must run all secondary questions")
			switch scenario {
			case "missing", "http-error":
				require.Error(t, err)
				require.Equal(t, "ERROR", result.Status)
			case "no-tags":
				require.NoError(t, err)
				require.Equal(t, "OK", result.Status)
				require.Empty(t, result.Level2)
			case "uncertain":
				require.NoError(t, err)
				require.Equal(t, "REVIEW", result.Status)
				require.Len(t, result.Level2, 1, "uncertain secondary answers still retain the highest candidate")
			default:
				require.NoError(t, err)
				require.Equal(t, "REVIEW", result.Status)
			}
		})
	}
}

func TestClassifyRejectsMalformedPrimaryAndCancelledRequest(t *testing.T) {
	for _, scenario := range []string{"missing-confidence", "bad-sum", "invalid-choice", "missing-probability", "null-probability", "wrong-model", "invalid-json"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				answer := primaryAnswer("SAFE_READ")
				model := Model
				switch scenario {
				case "missing-confidence":
					delete(answer, "confidence")
				case "bad-sum":
					answer["probabilities"].(map[string]float64)["UNKNOWN"] = 0.99
				case "invalid-choice":
					answer["choice"] = "safe"
				case "missing-probability":
					delete(answer["probabilities"].(map[string]float64), "UNKNOWN")
				case "null-probability":
					values := map[string]any{}
					for key, p := range answer["probabilities"].(map[string]float64) {
						values[key] = p
					}
					values["UNKNOWN"] = nil
					answer["probabilities"] = values
				case "wrong-model":
					model = "other-model"
				case "invalid-json":
					_, _ = w.Write([]byte("invalid"))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"model": model, "answers": map[string]any{"level1": answer}})
			}))
			defer server.Close()
			client := New("test-key")
			client.Endpoint = server.URL
			result, err := client.Classify(context.Background(), State{Command: "uname -a"})
			require.Error(t, err)
			require.Equal(t, "ERROR", result.Status)
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = client.Classify(ctx, State{Command: "uname -a"})
			require.Error(t, err)
		})
	}
}
