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
	"github.com/hpcsc/vet/internal/questions"
)

// maxAttempts bounds the retries the client makes before giving up.
const maxAttempts = 6

type Client struct {
	http  *http.Client
	api   string
	model string
	key   string
}

func NewClient(httpClient *http.Client, api, model, key string) *Client {
	return &Client{http: httpClient, api: api, model: model, key: key}
}

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
	Type       questions.Kind `json:"type"`
	Noul       *float64       `json:"noul"`
	Choice     *string        `json:"choice"`
	Score      *float64       `json:"score"`
	Confidence *float64       `json:"confidence"`
}

func (c *Client) Ask(ctx context.Context, state backend.State, file questions.File) ([]backend.Answer, error) {
	req, err := c.request(state, file)
	if err != nil {
		return nil, err
	}
	resp, err := c.post(ctx, req)
	if err != nil {
		return nil, err
	}
	return c.answersInto(resp, file)
}

func (c *Client) request(state backend.State, file questions.File) (request, error) {
	questionsMap := make(map[string]question, len(file.Rules))
	for _, rule := range file.Rules {
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
	return request{
		State:     "File: " + state.Path + "\n\n" + state.Diff,
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

// wait sleeps before the next attempt. It honors Retry-After in seconds when
// the server sends it, and falls back to exponential backoff. It reports
// false when the context ends, or when the client gave up.
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
	a := backend.Answer{Rule: rule.ID, Confidence: nil}
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
