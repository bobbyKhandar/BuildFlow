package server

import (
	"net/http"
	"strings"

	"buildflow/internal/llm"
)

type llmConfigRequest struct {
	Mode     string `json:"mode"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type llmKeyRequest struct {
	Provider string `json:"provider"`
	Key      string `json:"key"`
}

type llmChatRequest struct {
	Messages []llm.Message `json:"messages"`
}

func (s *Server) handleGetLLMConfig(w http.ResponseWriter, _ *http.Request) {
	manager, ok := s.llmManager(w)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, manager.Config())
}

func (s *Server) handleSetLLMConfig(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.llmManager(w)
	if !ok {
		return
	}
	var req llmConfigRequest
	if !readBody(w, r, &req) {
		return
	}
	if err := manager.SetConfig(req.Mode, req.Provider, req.Model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, manager.Config())
}

func (s *Server) handleSetLLMKey(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.llmManager(w)
	if !ok {
		return
	}
	var req llmKeyRequest
	if !readBody(w, r, &req) {
		return
	}
	if err := manager.SetKey(req.Provider, req.Key); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, manager.Config())
}

func (s *Server) handleRemoveLLMKey(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.llmManager(w)
	if !ok {
		return
	}
	if err := manager.RemoveKey(r.URL.Query().Get("provider")); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, manager.Config())
}

func (s *Server) handleLLMModels(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.llmManager(w)
	if !ok {
		return
	}
	provider := strings.TrimSpace(r.URL.Query().Get("provider"))
	if provider == "" {
		provider = manager.Config().Provider
	}
	models, err := manager.Models(r.Context(), provider)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	limits := map[string]llm.RateLimits{}
	if provider == "groq" {
		for _, model := range models {
			if limit := llm.PublishedGroqFreeLimits(model); limit != nil {
				limits[model] = *limit
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"provider": provider, "models": models, "publishedFreeLimits": limits})
}

func (s *Server) handleLLMChat(w http.ResponseWriter, r *http.Request) {
	manager, ok := s.llmManager(w)
	if !ok {
		return
	}
	var req llmChatRequest
	if !readBody(w, r, &req) {
		return
	}
	result, err := manager.Complete(r.Context(), req.Messages)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) llmManager(w http.ResponseWriter) (*llm.Manager, bool) {
	if s.app.LLM == nil {
		writeError(w, http.StatusServiceUnavailable, "language model service is unavailable")
		return nil, false
	}
	return s.app.LLM, true
}
