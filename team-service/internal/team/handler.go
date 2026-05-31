package team

import (
	"encoding/json"
	"errors"
	"net/http"

	"team-service/internal/platform/auth"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /teams", h.createTeam)
	mux.HandleFunc("GET /teams/{id}/members", h.listMembers)
	mux.HandleFunc("POST /teams/{id}/members", h.addMember)
	mux.HandleFunc("DELETE /teams/{id}/members/{userId}", h.removeMember)
	mux.HandleFunc("POST /teams/{id}/managers", h.addManager)
}

func (h *Handler) listMembers(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.RequirePrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	members, err := h.service.ListMembers(r.Context(), actor, r.PathValue("id"))
	if err != nil {
		handleTeamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"members": members})
}

func (h *Handler) createTeam(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.RequirePrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req CreateTeamRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	team, err := h.service.CreateTeam(r.Context(), actor, req)
	if err != nil {
		handleTeamError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"team": team})
}

func (h *Handler) addMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.RequirePrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req TeamUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	team, err := h.service.AddMember(r.Context(), actor, r.PathValue("id"), req)
	if err != nil {
		handleTeamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team": team})
}

func (h *Handler) removeMember(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.RequirePrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	team, err := h.service.RemoveMember(r.Context(), actor, r.PathValue("id"), r.PathValue("userId"))
	if err != nil {
		handleTeamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team": team})
}

func (h *Handler) addManager(w http.ResponseWriter, r *http.Request) {
	actor, ok := auth.RequirePrincipal(r.Context())
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req TeamUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	team, err := h.service.AddManager(r.Context(), actor, r.PathValue("id"), req)
	if err != nil {
		handleTeamError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"team": team})
}

func handleTeamError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
	case errors.Is(err, ErrForbidden):
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
	case errors.Is(err, ErrTeamNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "team not found"})
	case errors.Is(err, ErrMemberAlreadySet):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "member already exists"})
	case errors.Is(err, ErrMemberNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "member not found"})
	case errors.Is(err, ErrManagerAlreadySet):
		writeJSON(w, http.StatusConflict, map[string]string{"error": "manager already exists"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
