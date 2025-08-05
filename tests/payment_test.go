package tests_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/golang-queue/queue"
	"github.com/golang-queue/queue/core"
	"github.com/google/uuid"
	"github.com/lauro-santana/rinha-backend-2025/internal/handler"
	"github.com/lauro-santana/rinha-backend-2025/internal/repository/database"
	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
	"github.com/shopspring/decimal"
)

type AdminPaymentSummary struct {
	TotalRequests     uint            `json:"totalRequests"`
	TotalAmount       decimal.Decimal `json:"totalAmount"`
	TotalFee          decimal.Decimal `json:"totalFee"`
	FeePerTransaction decimal.Decimal `json:"feePerTransaction"`
}

func TestPayment(t *testing.T) {
	db, err := database.NewDatabase("localhost", "5433", "1")
	if err != nil {
		panic(err)
	}

	channel := make(chan model.Payment, 100)
	pool := queue.NewPool(10, queue.WithFn(func(ctx context.Context, m core.TaskMessage) error {
		var v model.Payment
		if err := json.Unmarshal(m.Payload(), &v); err != nil {
			return err
		}

		channel <- v
		return nil
	}))

	paymentConsumer := service.NewPaymentConsumer(os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT"), os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK"), db, pool, channel)
	go paymentConsumer.StartPaymentConsumer()

	handlerPayment := handler.NewPayment(service.NewPayment(db, pool))

	defaultHost, _ := os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT"), os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK")

	testCases := []struct {
		desc     string
		testCase func(t *testing.T)
	}{
		{
			desc: "post one payment",
			testCase: func(t *testing.T) {
				payment := model.Payment{
					CorrelationId: uuid.New().String(),
					Amount:        decimal.NewFromFloatWithExponent(19.23, -2),
				}
				pb, err := json.Marshal(payment)
				if err != nil {
					t.Errorf("expected marshal payment got error %v", err)
				}
				from := time.Now().Format(time.RFC3339)
				req := httptest.NewRequest(http.MethodPost, "/payments", bytes.NewBuffer(pb))
				rec := httptest.NewRecorder()

				handlerPayment.Post(rec, req)
				if rec.Code != 200 {
					t.Errorf("expected 200 got %v", rec.Code)
				}

				time.Sleep(1 * time.Second)

				err = validateRequest(handlerPayment, defaultHost, from)
				if err != nil {
					t.Error(err)
				}
			},
		},
		{
			desc: "post empty body",
			testCase: func(t *testing.T) {
				req := httptest.NewRequest(http.MethodPost, "/payments", nil)
				rec := httptest.NewRecorder()

				handlerPayment.Post(rec, req)
				if rec.Code != 400 {
					t.Errorf("expected 400 got %v", rec.Code)
				}
			},
		},
		{
			desc: "post mult values",
			testCase: func(t *testing.T) {
				from := time.Now().Format(time.RFC3339)
				var wg sync.WaitGroup
				for range 10 {
					wg.Add(1)
					go func() {
						payment := model.Payment{
							CorrelationId: uuid.New().String(),
							Amount:        decimal.NewFromFloatWithExponent(20, -2),
						}
						pb, err := json.Marshal(payment)
						if err != nil {
							t.Errorf("expected marshal payment got error %v", err)
						}
						req := httptest.NewRequest(http.MethodGet, "/payments", bytes.NewBuffer(pb))
						rec := httptest.NewRecorder()

						handlerPayment.Post(rec, req)
						if rec.Code != 200 {
							t.Errorf("expected 200 got %v", rec.Code)
						}
						wg.Done()
					}()
				}
				wg.Wait()
				time.Sleep(1 * time.Second)
				err = validateRequest(handlerPayment, defaultHost, from)
				if err != nil {
					t.Error(err)
				}
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, tC.testCase)
	}
}

func validateRequest(handlerPayment *handler.Handler, host, from string) error {
	req := httptest.NewRequest(http.MethodGet, "/payments-summary", nil)
	query := req.URL.Query()
	to := time.Now().Format(time.RFC3339)
	query.Set("from", from)
	query.Set("to", to)
	req.URL.RawQuery = query.Encode()

	rec := httptest.NewRecorder()
	handlerPayment.Get(rec, req)
	if rec.Code != 200 {
		return fmt.Errorf("expected 200 got %v", rec.Code)
	}
	var paymentSummary model.PaymentSummary
	err := json.NewDecoder(rec.Body).Decode(&paymentSummary)
	if err != nil {
		return fmt.Errorf("expected decode payment summary got error %v", err)
	}

	return validateSummary(host, from, to, paymentSummary.Default)
}

func validateSummary(host, from, to string, summary model.Summary) error {
	req, err := http.NewRequest(http.MethodGet, host+"/admin/payments-summary", nil)
	if err != nil {
		return fmt.Errorf("expected new request got error %v", err)
	}
	query := req.URL.Query()
	query.Set("from", from)
	query.Set("to", to)
	req.URL.RawQuery = query.Encode()
	req.Header.Set("X-Rinha-Token", "123")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("expected response got error %v", err)
	}

	if res.StatusCode != 200 {
		return fmt.Errorf("expected 200 status got error %v", res.StatusCode)
	}
	var adminSummary AdminPaymentSummary
	err = json.NewDecoder(res.Body).Decode(&adminSummary)
	if err != nil {
		return fmt.Errorf("expected decode got error %v", err)
	}

	if adminSummary.TotalRequests != summary.TotalRequests {
		return fmt.Errorf("expected %v got %v", adminSummary.TotalRequests, summary.TotalRequests)
	}

	if !adminSummary.TotalAmount.Equal(summary.TotalAmount) {
		return fmt.Errorf("expected %v got %v", adminSummary.TotalAmount, summary.TotalAmount)
	}

	return nil
}
