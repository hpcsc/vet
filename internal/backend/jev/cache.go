package jev

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
)

// cache stores a response for a request it has seen, so the same prompt gets
// the same answer across runs. Jev answers the same question differently on
// different calls, so without this a re-run of the same commit churns.
type cache struct {
	dir string
}

func newCache(dir string) *cache {
	return &cache{dir: dir}
}

func (c *cache) load(key string) (response, bool) {
	raw, err := os.ReadFile(filepath.Join(c.dir, key+".json"))
	if err != nil {
		return response{}, false
	}
	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return response{}, false
	}
	return r, true
}

func (c *cache) store(key string, r response) {
	if err := os.MkdirAll(c.dir, 0o755); err != nil {
		return
	}
	raw, err := json.Marshal(r)
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(c.dir, key+".json"), raw, 0o644)
}

// requestKey names a request by its content, so two requests with the same
// model, prompt and questions share an answer. The questions map is marshaled
// in key order, so the same request always hashes the same way.
func requestKey(req request) (string, error) {
	raw, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
