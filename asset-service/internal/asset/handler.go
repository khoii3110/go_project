package asset

import (
	"encoding/json"
	"errors"
	"net/http"
)

type Handler struct { service *Service }

func NewHandler(service *Service) *Handler { return &Handler{service: service} }

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /assets/{id}/metadata", h.getMetadata)
	mux.HandleFunc("PUT /assets/{id}/metadata", h.updateMetadata)
	mux.HandleFunc("GET /assets/{id}/acl", h.getACL)
	mux.HandleFunc("PUT /assets/{id}/acl", h.setACL)
	mux.HandleFunc("POST /folders/{id}/share", h.shareFolder)
	mux.HandleFunc("PUT /notes/{id}", h.updateNote)
}

func (h *Handler) getMetadata(w http.ResponseWriter, r *http.Request) {
	metadata, err := h.service.GetMetadata(r.Context(), r.PathValue("id"))
	if err != nil { handleAssetError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"metadata": metadata})
}

func (h *Handler) updateMetadata(w http.ResponseWriter, r *http.Request) {
	var metadata json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&metadata); err != nil { writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"}); return }
	updated, err := h.service.UpdateMetadata(r.Context(), r.PathValue("id"), metadata)
	if err != nil { handleAssetError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]json.RawMessage{"metadata": updated})
}

func (h *Handler) getACL(w http.ResponseWriter, r *http.Request) {
	acl, err := h.service.GetACL(r.Context(), r.PathValue("id"))
	if err != nil { handleAssetError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"acl": acl})
}

func (h *Handler) setACL(w http.ResponseWriter, r *http.Request) {
	var req struct { UserIDs []string `json:"userIds"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"}); return }
	acl, err := h.service.SetACL(r.Context(), r.PathValue("id"), req.UserIDs)
	if err != nil { handleAssetError(w, err); return }
	writeJSON(w, http.StatusOK, map[string]any{"acl": acl})
}

func (h *Handler) shareFolder(w http.ResponseWriter, r *http.Request) {
	if err := h.service.ShareFolder(r.Context(), r.PathValue("id")); err != nil { handleAssetError(w, err); return }
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "folder shared"})
}

func (h *Handler) updateNote(w http.ResponseWriter, r *http.Request) {
	if err := h.service.UpdateNote(r.Context(), r.PathValue("id")); err != nil { handleAssetError(w, err); return }
	writeJSON(w, http.StatusAccepted, map[string]string{"message": "note updated"})
}

func handleAssetError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidInput):
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
	case errors.Is(err, ErrAssetNotFound):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "asset not found"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
