package jev

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hpcsc/vet/internal/backend"
	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/questions"
)

const maxAttempts = 6

type Client struct {
	http  *http.Client
	api   string
	model string
	key   string
}

func NewClient(httpClient *http.Client, api, model, key string) backend.Judge {
	return &Client{http: httpClient, api: api, model: model, key: key}
}

var _ backend.Judge = (*Client)(nil)

type request struct {
	State     string              `json:"state"`
	Model     string              `json:"model"`
	Questions map[string]question `json:"questions"`
}

type question struct {
	Type         questions.Kind `json:"type"`
	Instructions string         `json:"instructions"`
	Criteria     any            `json:"criteria,omitempty"`
}

type response struct {
	Answers map[string]answer `json:"answers"`
}

type answer struct {
	Type          questions.Kind     `json:"type"`
	Noul          *float64           `json:"noul"`
	Choice        *string            `json:"choice"`
	Score         *float64           `json:"score"`
	Confidence    *float64           `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
	Legend        map[string]string  `json:"legend,omitempty"`
}

func (c *Client) Ask(ctx context.Context, file diff.File, q questions.File) ([]backend.Answer, error) {
	req, err := c.request(file, q)
	if err != nil {
		return nil, err
	}
	resp, err := c.post(ctx, req)
	if err != nil {
		return nil, err
	}
	return c.answersInto(resp, q)
}

func (c *Client) request(file diff.File, rules questions.File) (request, error) {
	questionsMap := make(map[string]question, len(rules.Rules))
	for _, rule := range rules.Rules {
		q := question{Type: rule.Type, Instructions: rule.Instructions}
		switch rule.Type {
		case questions.Noul:
		case questions.Choice:
			q.Criteria = rule.Choices
		case questions.Score:
			q.Criteria = rule.Scores
		default:
			return request{}, fmt.Errorf("rule %s has no supported type", rule.ID)
		}
		questionsMap[rule.ID] = q
	}
	state := "File: " + file.Path + "\n\n" + file.Diff
	if rules.Context != "" {
		state = rules.Context + "\n\n" + state
	}
	return request{
		State:     state,
		Model:     c.model,
		Questions: questionsMap,
	}, nil
}

func (c *Client) post(ctx context.Context, req request) (response, error) {
	var lastStatus int
	for attempt := 0; attempt < maxAttempts; attempt++ {
		body, err := json.Marshal(req)
		if err != nil {
			return response{}, err
		}
		httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, c.api, bytes.NewReader(body))
		if err != nil {
			return response{}, err
		}
		httpRequest.Header.Set("Content-Type", "application/json")
		if c.key != "" {
			httpRequest.Header.Set("Authorization", "Bearer "+c.key)
		}
		resp, err := c.http.Do(httpRequest)
		if err != nil {
			return response{}, err
		}
		responseBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return response{}, err
		}
		lastStatus = resp.StatusCode
		switch resp.StatusCode {
		case http.StatusOK:
			var r response
			if err := json.Unmarshal(responseBody, &r); err != nil {
				return response{}, fmt.Errorf("read the answer of %s: %w", c.model, err)
			}
			return r, nil
		case http.StatusTooManyRequests, 529:
			if !wait(ctx, resp.Header.Get("Retry-After"), attempt) {
				return response{}, fmt.Errorf("the backend kept answering %d", lastStatus)
			}
		default:
			return response{}, fmt.Errorf("the backend answered %s: %s", resp.Status, strings.TrimSpace(string(responseBody)))
		}
	}
	return response{}, fmt.Errorf("the backend kept answering %d", lastStatus)
}

func wait(ctx context.Context, retryAfter string, attempt int) bool {
	d, err := strconv.Atoi(retryAfter)
	if err != nil {
		d = 1 << uint(attempt)
	}
	if d > 0 {
		select {
		case <-time.After(time.Duration(d) * time.Second):
			return true
		case <-ctx.Done():
			return false
		}
	}
	return true
}

func (c *Client) answersInto(resp response, file questions.File) ([]backend.Answer, error) {
	answers := make([]backend.Answer, 0, len(file.Rules))
	for _, rule := range file.Rules {
		raw, ok := resp.Answers[rule.ID]
		if !ok {
			return nil, fmt.Errorf("the backend skipped rule %s", rule.ID)
		}
		answer, err := answerFor(rule, raw)
		if err != nil {
			return nil, err
		}
		answers = append(answers, answer)
	}
	return answers, nil
}

func answerFor(rule questions.Rule, raw answer) (backend.Answer, error) {
	a := backend.Answer{Rule: rule.ID, Confidence: nil, Probabilities: raw.Probabilities, Legend: raw.Legend}
	switch rule.Type {
	case questions.Noul:
		if raw.Noul == nil {
			return backend.Answer{}, fmt.Errorf("rule %s got no noul answer", rule.ID)
		}
		a.Noul = raw.Noul
	case questions.Choice:
		if raw.Choice == nil {
			return backend.Answer{}, fmt.Errorf("rule %s got no choice answer", rule.ID)
		}
		a.Choice = raw.Choice
		a.Confidence = raw.Confidence
	case questions.Score:
		if raw.Score == nil {
			return backend.Answer{}, fmt.Errorf("rule %s got no score answer", rule.ID)
		}
		rounded := int(math.Round(*raw.Score))
		a.Score = &rounded
		a.Confidence = raw.Confidence
	default:
		return backend.Answer{}, fmt.Errorf("rule %s has no supported type", rule.ID)
	}
	return a, nil
}
