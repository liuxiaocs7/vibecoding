package api

import (
	"net/http"

	"github.com/ymhhh/vibecoding/internal/llm"
	"github.com/ymhhh/vibecoding/internal/model"
)

func (s *Server) handleGetModel(w http.ResponseWriter, r *http.Request) {
	cfg, err := s.Store.GetModelConfig()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, cfg.Public())
}

func (s *Server) handlePutModel(w http.ResponseWriter, r *http.Request) {
	var cfg model.ModelConfig
	if err := decodeJSON(r, &cfg); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if cfg.OpenAIBaseURL != "" {
		if err := llm.ValidateBaseURL(cfg.OpenAIBaseURL); err != nil {
			writeErr(w, 400, err.Error())
			return
		}
	}
	if err := s.Store.PutModelConfig(cfg); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	saved, _ := s.Store.GetModelConfig()
	writeJSON(w, 200, saved.Public())
}

func (s *Server) handleGetUIPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := s.Store.GetUIPrefs()
	if err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, prefs)
}

func (s *Server) handlePutUIPrefs(w http.ResponseWriter, r *http.Request) {
	var prefs model.UIPrefs
	if err := decodeJSON(r, &prefs); err != nil {
		writeErr(w, 400, "invalid JSON")
		return
	}
	if err := s.Store.PutUIPrefs(prefs); err != nil {
		writeErr(w, 500, err.Error())
		return
	}
	writeJSON(w, 200, prefs)
}
