package handler

import (
	"api/internal/service"
	"encoding/json"
	"net/http"
	"strings"
)

type UserHandler struct {
	service *service.UserService
}

func NewUserHandler(s *service.UserService) *UserHandler {
	return &UserHandler{service: s}
}

func (h *UserHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/users/")
	if id == "" {
		http.Error(w, "missing user id", http.StatusBadRequest)
		return
	}

	user, err := h.service.GetUserProfile(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}
