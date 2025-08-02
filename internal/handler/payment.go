package handler

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
)

type Handler struct {
	service service.IPayment
}

func NewPayment(s service.IPayment) *Handler {
	return &Handler{s}
}

func (h *Handler) Post(w http.ResponseWriter, r *http.Request) {
	var payment model.Payment
	err := json.NewDecoder(r.Body).Decode(&payment)
	if err != nil {
		if errors.Is(err, io.EOF) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("error on decode payment", err)
		return
	}
	if payment.Amount.IsZero() || payment.Amount.IsNegative() || payment.CorrelationId == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	err = h.service.Post(payment)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	urlFrom := r.URL.Query().Get("from")
	urlTo := r.URL.Query().Get("to")

	if urlFrom == "" || urlTo == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	timeFrom, err := time.Parse(time.RFC3339, urlFrom)
	if err != nil {
		log.Printf("error %v on parse string time %v\n", err, urlFrom)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	timeTo, err := time.Parse(time.RFC3339, urlTo)
	if err != nil {
		log.Printf("error %v on parse string time %v\n", err, urlTo)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	paymentSummary, err := h.service.Get(timeFrom, timeTo)
	if err != nil {
		log.Printf("error %v on get payment summary\n", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err = json.NewEncoder(w).Encode(paymentSummary); err != nil {
		log.Printf("error %v on encode payment summary\n", err)
		w.WriteHeader(http.StatusInternalServerError)
	}
}
