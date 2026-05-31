package user

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /register", h.register)
	mux.HandleFunc("POST /login", h.login)
	mux.HandleFunc("POST /logout", h.logout)
	mux.HandleFunc("GET /users", h.listUsers)
	mux.HandleFunc("POST /import-users", h.importUsers)
}

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	u, err := h.service.Register(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		case errors.Is(err, ErrEmailAlreadyExists):
			writeJSON(w, http.StatusConflict, map[string]string{"error": "email already exists"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"user": u})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	resp, err := h.service.Login(r.Context(), req)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid input"})
		case errors.Is(err, ErrUnauthorized):
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	authCtx, err := h.requireAuth(r)
	if err != nil {
		handleAuthError(w, err)
		return
	}

	if err := h.service.Logout(r.Context(), authCtx.SessionID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "logged out"})
}

func (h *Handler) listUsers(w http.ResponseWriter, r *http.Request) {
	authCtx, err := h.requireAuth(r)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	if authCtx.Role != RoleManager {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "manager role required"})
		return
	}

	users, err := h.service.ListUsers(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

func (h *Handler) importUsers(w http.ResponseWriter, r *http.Request) {
	authCtx, err := h.requireAuth(r)
	if err != nil {
		handleAuthError(w, err)
		return
	}
	if authCtx.Role != RoleManager {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "manager role required"})
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "file field is required"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unable to read uploaded file"})
		return
	}

	summary, err := h.service.ImportUsers(r.Context(), ImportUsersRequest{
		Filename:    fileHeader.Filename,
		ContentType:  fileHeader.Header.Get("Content-Type"),
		CSVContents:  content,
		UploadedByID: authCtx.UserID,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidInput):
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid csv file"})
		case errors.Is(err, ErrForbidden):
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		default:
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handler) requireAuth(r *http.Request) (*AuthContext, error) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if authHeader == "" {
		return nil, ErrUnauthorized
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return nil, ErrInvalidToken
	}

	return h.service.Authenticate(r.Context(), parts[1])
}

func handleAuthError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidToken):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid token"})
	case errors.Is(err, ErrUnauthorized):
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	default:
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
	}
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
