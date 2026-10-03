package jev

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"time"
)

const (
	Endpoint      = "https://api.typesafe.ai/v1/systemone"
	Model         = "jev-1.13.0"
	PolicyVersion = "2026-09-28"
	// Conservative review thresholds; deployment samples must calibrate them.
	MinConfidence = 0.8
	NoulYes       = 0.8
	NoulNo        = 0.2
)

//go:embed rules.json
var rulesJSON []byte

type question struct {
	Type         string            `json:"type"`
	Instructions json.RawMessage   `json:"instructions"`
	Criteria     map[string]string `json:"criteria"`
}

var rules struct {
	Primary   map[string]question            `json:"primary"`
	Secondary map[string]map[string]question `json:"secondary"`
}

func init() {
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		panic(fmt.Sprintf("invalid Jev command rules: %v", err))
	}
}

type Classification struct {
	Level1        string          `json:"level1"`
	Level2        []string        `json:"level2"`
	Status        string          `json:"status"` // OK, REVIEW, ERROR, UNCONFIGURED, BLOCKED
	Model         string          `json:"model"`
	PolicyVersion string          `json:"policy_version"`
	Reason        string          `json:"reason,omitempty"`
	Primary       json.RawMessage `json:"primary,omitempty"`
	Secondary     json.RawMessage `json:"secondary,omitempty"`
}

func Unclassified(status, reason string) *Classification {
	return &Classification{Level1: "UNKNOWN", Level2: []string{}, Status: status, Model: Model, PolicyVersion: PolicyVersion, Reason: reason}
}

// State contains gateway-owned context only, never the agent's safety assertions.
type State struct {
	Command     string         `json:"command"`
	Context     map[string]any `json:"context"`
	PriorLevel1 string         `json:"prior_level1,omitempty"`
}

type Client struct {
	HTTP     *http.Client
	Endpoint string
	APIKey   string
}

func New(apiKey string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, Endpoint: Endpoint, APIKey: apiKey}
}

type response struct {
	Model   string                     `json:"model"`
	Answers map[string]json.RawMessage `json:"answers"`
}

func (c *Client) ask(ctx context.Context, state State, questions map[string]question) (result *response, err error) {
	body, err := json.Marshal(struct {
		Model     string              `json:"model"`
		State     State               `json:"state"`
		Questions map[string]question `json:"questions"`
	}{Model, state, questions})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.Endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Content-Type", "application/json")
	r, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jev request failed: %w", err)
	}
	defer func() {
		if closeErr := r.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("close jev response: %w", closeErr)
		}
	}()
	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("jev returned HTTP %d", r.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 1024*1024+1))
	if err != nil {
		return nil, fmt.Errorf("read Jev response: %w", err)
	}
	if len(data) > 1024*1024 {
		return nil, fmt.Errorf("jev response exceeds size limit")
	}
	var out response
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, fmt.Errorf("invalid Jev response: %w", err)
	}
	if out.Model != Model || len(out.Answers) != len(questions) {
		return &out, fmt.Errorf("unexpected Jev model or answer count")
	}
	return &out, nil
}

func probability(p float64) bool { return !math.IsNaN(p) && !math.IsInf(p, 0) && p >= 0 && p <= 1 }

func (c *Client) Classify(ctx context.Context, state State) (*Classification, error) {
	result := Unclassified("ERROR", "")
	if len(state.Command) > 32768 {
		return result, fmt.Errorf("command exceeds Jev classification size limit")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	first, err := c.ask(ctx, state, rules.Primary)
	if first != nil {
		result.Primary = first.Answers["level1"]
		result.Model = first.Model
	}
	if err != nil {
		return result, err
	}
	var answer struct {
		Choice        string              `json:"choice"`
		Confidence    *float64            `json:"confidence"`
		Probabilities map[string]*float64 `json:"probabilities"`
	}
	if err := json.Unmarshal(result.Primary, &answer); err != nil {
		return result, fmt.Errorf("invalid Jev primary answer: %w", err)
	}
	criteria := rules.Primary["level1"].Criteria
	if _, ok := criteria[answer.Choice]; !ok || answer.Confidence == nil || !probability(*answer.Confidence) || len(answer.Probabilities) != len(criteria) {
		return result, fmt.Errorf("incomplete Jev primary answer")
	}
	chosen, ok := answer.Probabilities[answer.Choice]
	if !ok || chosen == nil || !probability(*chosen) {
		return result, fmt.Errorf("invalid Jev selected probability")
	}
	sum := 0.0
	for key := range criteria {
		p, ok := answer.Probabilities[key]
		if !ok || p == nil || !probability(*p) || *p > *chosen+1e-6 {
			return result, fmt.Errorf("invalid Jev primary probabilities")
		}
		sum += *p
	}
	if math.Abs(sum-1) > 0.02 {
		return result, fmt.Errorf("invalid Jev primary probability sum")
	}
	result.Level1 = answer.Choice
	result.Status = "OK"
	if answer.Choice == "UNKNOWN" || *answer.Confidence < MinConfidence || *chosen < MinConfidence {
		result.Status = "REVIEW"
		result.Reason = "primary classification requires review"
	}
	questions, hasSecondary := rules.Secondary[answer.Choice]
	if !hasSecondary {
		return result, nil
	}
	state.PriorLevel1 = answer.Choice
	second, err := c.ask(ctx, state, questions)
	if err != nil {
		result.Status = "ERROR"
		result.Level2 = nil
		return result, err
	}
	result.Secondary, err = json.Marshal(second.Answers)
	if err != nil {
		result.Status = "ERROR"
		return result, err
	}
	for key := range questions {
		var a struct {
			Noul *float64 `json:"noul"`
		}
		if err := json.Unmarshal(second.Answers[key], &a); err != nil || a.Noul == nil || !probability(*a.Noul) {
			result.Status = "ERROR"
			result.Level2 = nil
			return result, fmt.Errorf("invalid Jev secondary answer %s", key)
		}
		if *a.Noul >= NoulYes {
			result.Level2 = append(result.Level2, key)
		}
		if *a.Noul > NoulNo && *a.Noul < NoulYes {
			result.Status = "REVIEW"
			result.Reason = "secondary classification requires review"
		}
	}
	sort.Strings(result.Level2)
	return result, nil
}
