package handler

import (
	"net/http"

	"github.com/google/uuid"
)

func (h *Handler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	h.mu.RLock()
	event, ok := h.events[id]
	h.mu.RUnlock()
	if !ok {
		http.Error(w, "Такого события не существует", http.StatusNotFound)
		return
	}

	writeJSON(w, http.StatusOK, event)
}
