//go:build unit

package jev_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hpcsc/vet/internal/backend/jev"
	"github.com/hpcsc/vet/internal/diff"
	"github.com/hpcsc/vet/internal/material"
	"github.com/hpcsc/vet/internal/questions"
	"github.com/stretchr/testify/require"
)

func mustParse(t *testing.T, text string) questions.File {
	t.Helper()

	file, err := questions.Parse([]byte(text), "")
	require.NoError(t, err)
	return file
}

func TestClient(t *testing.T) {
	ctx := context.Background()

	questionsFile := mustParse(t, `version: 1
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
  - id: database-migration
    instructions: describes the change
    type: choice
    choices:
      no-db: nothing
      uses-db: reads or writes
      migrates: alters the schema
    violatesWhen: migrates
  - id: log-guideline
    instructions: rates the change
    type: score
    scores:
      - first
      - second
      - third
    scoreLimit: 2
`)

	noulFile := mustParse(t, `version: 1
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
`)

	file := diff.File{Path: "a.go", Diff: "@@ -1 +1 @@"}

	t.Run("ask", func(t *testing.T) {
		t.Run("puts the context above the file in the state", func(t *testing.T) {
			contextFile := mustParse(t, `version: 1
context: use slog
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
`)
			var got map[string]any
			server := newServer(t, `{"no-flag-field":{"type":"noul","noul":0.2}}`, &got)

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, contextFile, nil)

			require.NoError(t, err)
			require.Equal(t, "use slog\n\nFile: a.go\n\n@@ -1 +1 @@", got["state"])
		})

		t.Run("puts the material after the diff, so the change being judged stays last", func(t *testing.T) {
			var got map[string]any
			server := newServer(t, `{"no-flag-field":{"type":"noul","noul":0.2}}`, &got)
			sections := []material.Section{{
				Include: questions.SiblingFilePaths,
				Content: "Files in this directory:\ninternal/diff/file.go",
			}}

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, sections)

			require.NoError(t, err)
			require.Equal(t,
				"File: a.go\n\n@@ -1 +1 @@\n\nRepository material for a.go (siblingFilePaths):\n\nFiles in this directory:\ninternal/diff/file.go",
				got["state"])
		})

		t.Run("puts the material under the context, so the guideline still leads", func(t *testing.T) {
			var got map[string]any
			server := newServer(t, `{"no-flag-field":{"type":"noul","noul":0.2}}`, &got)
			contextFile := mustParse(t, `version: 1
context: use slog
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
`)
			sections := []material.Section{{Include: questions.FileContent, Content: "package a"}}

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, contextFile, sections)

			require.NoError(t, err)
			require.Equal(t,
				"use slog\n\nFile: a.go\n\n@@ -1 +1 @@\n\nRepository material for a.go (fileContent):\n\npackage a",
				got["state"])
		})

		t.Run("leaves the state alone when there is no material", func(t *testing.T) {
			var got map[string]any
			server := newServer(t, `{"no-flag-field":{"type":"noul","noul":0.2}}`, &got)

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, []material.Section{})

			require.NoError(t, err)
			require.Equal(t, "File: a.go\n\n@@ -1 +1 @@", got["state"])
		})

		t.Run("posts the file, the model and the questions to the endpoint", func(t *testing.T) {
			var got map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/v1/systemone", r.URL.Path)
				body, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				require.NoError(t, json.Unmarshal(body, &got))
				w.Header().Set("content-type", "application/json")
				_, err = io.WriteString(w, `{"answers":{"no-flag-field":{"type":"noul","noul":0.2}}}`)
				require.NoError(t, err)
			}))
			defer server.Close()
			client := jev.NewClient(server.Client(), server.URL+"/v1/systemone", "jev-latest", "secret")

			_, err := client.Ask(ctx, file, noulFile, nil)

			require.NoError(t, err)
			require.Equal(t, "File: a.go\n\n@@ -1 +1 @@", got["state"])
			require.Equal(t, "jev-latest", got["model"])
			question, ok := got["questions"].(map[string]any)["no-flag-field"].(map[string]any)
			require.True(t, ok)
			require.Equal(t, "noul", question["type"])
			require.Equal(t, "adds a flag field", question["instructions"])
		})

		t.Run("maps a choice rule to a criteria map", func(t *testing.T) {
			choice := buildQuestions(t, "choice", `    choices:
      no-db: nothing
      uses-db: reads or writes
    violatesWhen: uses-db`)
			var got map[string]any
			server := newServer(t, `{"only-rule":{"type":"choice","choice":"no-db"}}`, &got)

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, choice, nil)

			require.NoError(t, err)
			question := got["questions"].(map[string]any)["only-rule"].(map[string]any)
			require.Equal(t, "choice", question["type"])
			require.Equal(t, map[string]any{"no-db": "nothing", "uses-db": "reads or writes"}, question["criteria"])
		})

		t.Run("maps a score rule to an ordered criteria list", func(t *testing.T) {
			score := buildQuestions(t, "score", `    scores:
      - first
      - second
      - third
    scoreLimit: 1`)
			var got map[string]any
			server := newServer(t, `{"only-rule":{"type":"score","score":1}}`, &got)

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, score, nil)

			require.NoError(t, err)
			question := got["questions"].(map[string]any)["only-rule"].(map[string]any)
			require.Equal(t, "score", question["type"])
			require.Equal(t, []any{"first", "second", "third"}, question["criteria"])
		})

		t.Run("sends the API key on the request", func(t *testing.T) {
			var authorization string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				authorization = r.Header.Get("Authorization")
				w.Header().Set("content-type", "application/json")
				_, _ = io.WriteString(w, `{"answers":{"no-flag-field":{"type":"noul","noul":0.2}}}`)
			}))
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, nil)

			require.NoError(t, err)
			require.Equal(t, "Bearer secret", authorization)
		})

		t.Run("reads one answer per rule in file order", func(t *testing.T) {
			server := newServer(t, `{
				"no-flag-field":{"type":"noul","noul":0.2},
				"database-migration":{"type":"choice","choice":"uses-db","confidence":0.92},
				"log-guideline":{"type":"score","score":2}
			}`, nil)
			defer server.Close()

			answers, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, questionsFile, nil)

			require.NoError(t, err)
			require.Len(t, answers, 3)
			require.Equal(t, "no-flag-field", answers[0].Rule)
			require.Equal(t, 0.2, *answers[0].Noul)
			require.Nil(t, answers[0].Confidence)
			require.Equal(t, "database-migration", answers[1].Rule)
			require.Equal(t, "uses-db", *answers[1].Choice)
			require.Equal(t, 0.92, *answers[1].Confidence)
			require.Equal(t, "log-guideline", answers[2].Rule)
			require.Equal(t, 2, *answers[2].Score)
		})

		t.Run("reads the probabilities and legend the backend returns", func(t *testing.T) {
			server := newServer(t, `{
				"no-flag-field":{"type":"noul","noul":0.2},
				"database-migration":{"type":"choice","choice":"migrates","probabilities":{"no-db":0.0,"uses-db":0.05,"migrates":0.95},"confidence":0.92},
				"log-guideline":{"type":"score","score":2,"probabilities":{"0":0.05,"1":0.3,"2":0.65},"legend":{"0":"first","1":"second","2":"third"},"confidence":0.78}
			}`, nil)
			defer server.Close()

			answers, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, questionsFile, nil)

			require.NoError(t, err)
			require.Len(t, answers, 3)
			require.Equal(t, map[string]float64{"no-db": 0.0, "uses-db": 0.05, "migrates": 0.95}, answers[1].Probabilities)
			require.Empty(t, answers[1].Legend)
			require.Equal(t, map[string]float64{"0": 0.05, "1": 0.3, "2": 0.65}, answers[2].Probabilities)
			require.Equal(t, map[string]string{"0": "first", "1": "second", "2": "third"}, answers[2].Legend)
		})

		t.Run("rounds a fractional score down", func(t *testing.T) {
			scoreFile := buildQuestions(t, "score", `    scores:
      - first
      - second
      - third
    scoreLimit: 1`)
			server := newServer(t, `{"only-rule":{"type":"score","score":2.4}}`, nil)
			defer server.Close()

			answers, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, scoreFile, nil)

			require.NoError(t, err)
			require.Equal(t, 2, *answers[0].Score)
		})

		t.Run("fails when the response skips a rule", func(t *testing.T) {
			server := newServer(t, `{}`, nil)
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, questionsFile, nil)

			require.ErrorContains(t, err, "no-flag-field")
		})
	})

	t.Run("retries", func(t *testing.T) {
		t.Run("a 429 retries and honors the seconds of Retry-After", func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts < 3 {
					w.Header().Set("Retry-After", "0")
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.Header().Set("content-type", "application/json")
				_, _ = io.WriteString(w, `{"answers":{"no-flag-field":{"type":"noul","noul":0.2}}}`)
			}))
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, nil)

			require.NoError(t, err)
			require.Equal(t, 3, attempts)
		})

		t.Run("a 529 retries", func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				if attempts == 1 {
					w.WriteHeader(529)
					return
				}
				w.Header().Set("content-type", "application/json")
				_, _ = io.WriteString(w, `{"answers":{"no-flag-field":{"type":"noul","noul":0.2}}}`)
			}))
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, nil)

			require.NoError(t, err)
			require.Equal(t, 2, attempts)
		})

		t.Run("retries no more than six attempts", func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, nil)

			require.ErrorContains(t, err, "429")
			require.Equal(t, 6, attempts)
		})

		t.Run("a 401 fails at once with the server reason", func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = io.WriteString(w, `{"error":"the key is invalid"}`)
			}))
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, nil)

			require.ErrorContains(t, err, "the key is invalid")
			require.Equal(t, 1, attempts)
		})

		t.Run("a 422 fails at once with the server reason", func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				attempts++
				w.Header().Set("content-type", "application/json")
				w.WriteHeader(422)
				_, _ = io.WriteString(w, `{"error":"a question is malformed"}`)
			}))
			defer server.Close()

			_, err := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").Ask(ctx, file, noulFile, nil)

			require.ErrorContains(t, err, "a question is malformed")
			require.Equal(t, 1, attempts)
		})
	})
}

func TestCache(t *testing.T) {
	ctx := context.Background()
	noulFile := mustParse(t, `version: 1
rules:
  - id: no-flag-field
    instructions: adds a flag field
    type: noul
    noulLimit: 0.5
`)
	file := diff.File{Path: "a.go", Diff: "@@ -1 +1 @@"}

	t.Run("answers a repeated request from the cache", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.Header().Set("content-type", "application/json")
			_, _ = io.WriteString(w, `{"answers":{"no-flag-field":{"type":"noul","noul":0.2}}}`)
		}))
		defer server.Close()
		client := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").WithCache(t.TempDir())

		first, err := client.Ask(ctx, file, noulFile, nil)
		require.NoError(t, err)
		second, err := client.Ask(ctx, file, noulFile, nil)
		require.NoError(t, err)

		require.Equal(t, 1, requests)
		require.Equal(t, first, second)
	})

	t.Run("answers a different request from the model", func(t *testing.T) {
		requests := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			w.Header().Set("content-type", "application/json")
			_, _ = io.WriteString(w, `{"answers":{"no-flag-field":{"type":"noul","noul":0.2}}}`)
		}))
		defer server.Close()
		client := jev.NewClient(server.Client(), server.URL, "jev-latest", "secret").WithCache(t.TempDir())

		_, err := client.Ask(ctx, file, noulFile, nil)
		require.NoError(t, err)
		_, err = client.Ask(ctx, diff.File{Path: "b.go", Diff: "@@ -1 +1 @@"}, noulFile, nil)
		require.NoError(t, err)

		require.Equal(t, 2, requests)
	})
}

func buildQuestions(t *testing.T, kind, extra string) questions.File {
	t.Helper()

	file, err := questions.Parse([]byte("version: 1\nrules:\n  - id: only-rule\n    instructions: a rule\n    type: " + kind + "\n" + extra + "\n"), "")
	require.NoError(t, err)
	return file
}

func newServer(t *testing.T, answers string, got *map[string]any) *httptest.Server {
	t.Helper()

	body := `{"answers":` + answers + `}`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got != nil {
			raw, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(raw, got))
		}
		w.Header().Set("content-type", "application/json")
		_, err := io.WriteString(w, body)
		require.NoError(t, err)
	}))
	t.Cleanup(server.Close)
	return server
}
