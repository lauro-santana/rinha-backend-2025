package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
)

type Handler struct {
	service service.Paymenter
}

func NewPayment(s service.Paymenter) *Handler {
	return &Handler{s}
}

func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	var payment model.Payment
	err := json.NewDecoder(r.Body).Decode(&payment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("error on decode payment", err)
		return
	}
	if payment.Amount.IsZero() || payment.CorrelationId == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	log.Println("produce payment:", payment)
	err = h.service.Post(payment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}
