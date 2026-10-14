// Package llm provides BuildFlow's provider-neutral language-model boundary.
// Provider adapters speak the common chat-completions API; the rest of
// BuildFlow never needs provider SDK types or credentials.
package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"buildflow/internal/appdata"
)

type Provider struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Config struct {
	Mode       string     `json:"mode"`
	Provider   string     `json:"provider"`
	Model      string     `json:"model"`
	KeyPresent bool       `json:"keyPresent"`
	KeyFile    string     `json:"keyFile"`
	Providers  []Provider `json:"providers"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Completion struct {
	Content string `json:"content"`
	Model   string `json:"model"`
}

type RateLimits struct {
	RPM int `json:"rpm"`
	RPD int `json:"rpd"`
	TPM int `json:"tpm"`
	TPD int `json:"tpd"`
}

// PublishedGroqFreeLimits returns the public free-tier defaults currently
// documented for common Groq text models. Account-level overrides can differ.
func PublishedGroqFreeLimits(model string) *RateLimits {
	switch strings.ToLower(model) {
	case "openai/gpt-oss-120b", "openai/gpt-oss-20b", "qwen/qwen3.8-27b":
		return &RateLimits{RPM: 30, RPD: 1000, TPM: 8000, TPD: 200000}
	default:
		return nil
	}
}

type providerSpec struct {
	Provider
	BaseURL string
	Mode    string
}

var providerSpecs = []providerSpec{
	{Provider: Provider{ID: "groq", Name: "Groq Cloud"}, BaseURL: "https://api.groq.com/openai/v1", Mode: "free"},
	{Provider: Provider{ID: "openai", Name: "OpenAI"}, BaseURL: "https://api.openai.com/v1", Mode: "premium"},
	{Provider: Provider{ID: "deepseek", Name: "DeepSeek"}, BaseURL: "https://api.deepseek.com/v1", Mode: "premium"},
	{Provider: Provider{ID: "mistral", Name: "Mistral"}, BaseURL: "https://api.mistral.ai/v1", Mode: "premium"},
	{Provider: Provider{ID: "openrouter", Name: "OpenRouter"}, BaseURL: "https://openrouter.ai/api/v1", Mode: "premium"},
}

type Manager struct {
	store  *appdata.Store
	client *http.Client
	mu     sync.Mutex
}

func New(store *appdata.Store) *Manager {
	return &Manager{store: store, client: &http.Client{Timeout: 45 * time.Second}}
}

func (m *Manager) Config() Config {
	s := m.store.Settings()
	mode, provider := normalizeSelection(s.LLMMode, s.LLMProvider)
	key, _ := m.readKey(provider)
	return Config{Mode: mode, Provider: provider, Model: s.LLMModel, KeyPresent: key != "", KeyFile: m.envPath(), Providers: providersFor(mode)}
}

func (m *Manager) SetConfig(mode, provider, model string) error {
	mode = strings.ToLower(strings.TrimSpace(mode))
	provider = strings.ToLower(strings.TrimSpace(provider))
	model = strings.TrimSpace(model)
	if mode == "" {
		mode = "free"
	}
	if mode != "free" && mode != "premium" {
		return errors.New("mode must be free or premium")
	}
	if len(model) > 200 || strings.ContainsAny(model, "\r\n") {
		return errors.New("model name is invalid")
	}
	if provider == "" {
		if mode == "free" {
			provider = "groq"
		} else {
			provider = "openai"
		}
	}
	if !providerAllowed(mode, provider) {
		return errors.New("provider is not available in this mode")
	}
	return m.store.Update(func(s *appdata.Settings) {
		s.LLMMode, s.LLMProvider, s.LLMModel = mode, provider, model
	})
}

func (m *Manager) SetKey(provider, key string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if specFor(provider) == nil {
		return errors.New("unknown provider")
	}
	key = strings.TrimSpace(key)
	if len(key) > 4096 || strings.ContainsAny(key, "\r\n\x00") {
		return errors.New("API key must be a single line of at most 4096 characters")
	}
	if key == "" {
		return errors.New("enter an API key, or use Remove to delete the saved key")
	}
	return m.writeKey(provider, key)
}

func (m *Manager) RemoveKey(provider string) error {
	provider = strings.ToLower(strings.TrimSpace(provider))
	if specFor(provider) == nil {
		return errors.New("unknown provider")
	}
	return m.writeKey(provider, "")
}

func (m *Manager) Models(ctx context.Context, provider string) ([]string, error) {
	spec := specFor(strings.ToLower(strings.TrimSpace(provider)))
	if spec == nil {
		return nil, errors.New("unknown provider")
	}
	key, err := m.readKey(spec.ID)
	if err != nil {
		return nil, fmt.Errorf("read saved API key: %w", err)
	}
	if key == "" {
		return nil, errors.New("add an API key for this provider first")
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, spec.BaseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("connect to %s: %w", spec.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, providerStatus(spec.Name, resp.StatusCode)
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("read model list from %s: %w", spec.Name, err)
	}
	models := make([]string, 0, len(payload.Data))
	for _, item := range payload.Data {
		id := strings.TrimSpace(item.ID)
		if id != "" && chatModel(id) {
			models = append(models, id)
		}
	}
	sort.Strings(models)
	if len(models) == 0 {
		return nil, fmt.Errorf("%s returned no chat models", spec.Name)
	}
	return models, nil
}

func (m *Manager) Complete(ctx context.Context, messages []Message) (Completion, error) {
	s := m.store.Settings()
	mode, provider := normalizeSelection(s.LLMMode, s.LLMProvider)
	if !providerAllowed(mode, provider) {
		return Completion{}, errors.New("selected provider is not available in this mode")
	}
	model := strings.TrimSpace(s.LLMModel)
	if model == "" {
		return Completion{}, errors.New("choose a model in Settings first")
	}
	if len(messages) == 0 || len(messages) > 100 {
		return Completion{}, errors.New("provide between 1 and 100 messages")
	}
	for _, msg := range messages {
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" {
			return Completion{}, errors.New("message role must be system, user, or assistant")
		}
		if len(msg.Content) > 1<<20 {
			return Completion{}, errors.New("a message is too large")
		}
	}
	spec := specFor(provider)
	key, err := m.readKey(provider)
	if err != nil {
		return Completion{}, err
	}
	if key == "" {
		return Completion{}, errors.New("add an API key for the selected provider in Settings")
	}
	body, err := json.Marshal(map[string]any{"model": model, "messages": messages})
	if err != nil {
		return Completion{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spec.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Completion{}, err
	}
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := m.client.Do(req)
	if err != nil {
		return Completion{}, fmt.Errorf("request to %s failed: %w", spec.Name, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Completion{}, providerStatus(spec.Name, resp.StatusCode)
	}
	var payload struct {
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 8<<20)).Decode(&payload); err != nil {
		return Completion{}, fmt.Errorf("read response from %s: %w", spec.Name, err)
	}
	if len(payload.Choices) == 0 {
		return Completion{}, fmt.Errorf("%s returned no answer", spec.Name)
	}
	return Completion{Content: payload.Choices[0].Message.Content, Model: payload.Model}, nil
}

func providersFor(mode string) []Provider {
	out := []Provider{}
	for _, spec := range providerSpecs {
		if spec.Mode == mode {
			out = append(out, spec.Provider)
		}
	}
	return out
}

func normalizeSelection(mode, provider string) (string, string) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	provider = strings.ToLower(strings.TrimSpace(provider))
	if mode != "premium" {
		mode = "free"
	}
	if !providerAllowed(mode, provider) {
		if mode == "free" {
			provider = "groq"
		} else {
			provider = "openai"
		}
	}
	return mode, provider
}

func providerAllowed(mode, id string) bool {
	spec := specFor(id)
	return spec != nil && spec.Mode == mode
}

func specFor(id string) *providerSpec {
	for i := range providerSpecs {
		if providerSpecs[i].ID == id {
			return &providerSpecs[i]
		}
	}
	return nil
}

func chatModel(id string) bool {
	lower := strings.ToLower(id)
	for _, blocked := range []string{"embed", "whisper", "speech", "transcrib", "tts", "moderation", "guard"} {
		if strings.Contains(lower, blocked) {
			return false
		}
	}
	return true
}

func providerStatus(name string, code int) error {
	// Never include a provider response body here: some services echo request
	// details, and API keys must not reach the UI or logs.
	if code == http.StatusTooManyRequests {
		return fmt.Errorf("%s rate limit reached; wait for the provider's reset window or check the account limits", name)
	}
	if code == http.StatusUnauthorized || code == http.StatusForbidden {
		return fmt.Errorf("%s rejected the API key (HTTP %d); check the saved key and account access", name, code)
	}
	return fmt.Errorf("%s rejected the request (HTTP %d); check the saved key and provider account", name, code)
}

func (m *Manager) readKey(provider string) (string, error) {
	path := m.envPath()
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	want := envKey(provider)
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimPrefix(line, "export ")
		name, value, ok := strings.Cut(line, "=")
		if ok && strings.TrimSpace(name) == want {
			value = strings.TrimSpace(value)
			if strings.HasPrefix(value, "\"") {
				if decoded, err := strconv.Unquote(value); err == nil {
					return decoded, nil
				}
			}
			return strings.Trim(value, "\"'"), nil
		}
	}
	return "", nil
}

func (m *Manager) writeKey(provider, value string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := m.envPath()
	raw, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read provider key file: %w", err)
	}
	lines := strings.Split(strings.TrimRight(string(raw), "\n"), "\n")
	want := envKey(provider)
	found := false
	out := make([]string, 0, len(lines)+1)
	for _, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "export "))
		name, _, ok := strings.Cut(trimmed, "=")
		if ok && strings.TrimSpace(name) == want {
			if value != "" && !found {
				out = append(out, want+"="+strconv.Quote(value))
			}
			found = true
			continue
		}
		if line != "" || len(lines) > 1 {
			out = append(out, line)
		}
	}
	if value != "" && !found {
		out = append(out, want+"="+strconv.Quote(value))
	}
	content := strings.Join(out, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("prepare state folder for API keys: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return fmt.Errorf("write API key file: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("secure API key file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("replace API key file: %w", err)
	}
	return nil
}

func (m *Manager) envPath() string { return m.store.StatePath(".env") }

func envKey(provider string) string {
	return "BUILDFLOW_LLM_KEY_" + strings.ToUpper(strings.ReplaceAll(provider, "-", "_"))
}
