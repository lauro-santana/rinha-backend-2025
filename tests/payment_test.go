package tests_test

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lauro-santana/rinha-backend-2025/internal/handler"
	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/shopspring/decimal"
)

type Teste struct {
	M model.Payment
}

func TestPayment(t *testing.T) {
	queueName := "job_queue"
	var conn *amqp.Connection
	var err error

	for i := range 10 {
		conn, err = amqp.Dial("amqp://guest:guest@localhost:5673/")
		if err == nil {
			log.Println("Connected to RabbitMQ")
			break
		}
		log.Printf("RabbitMQ not ready yet... retrying (%d/10)\n", i+1)
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		panic(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	//defer ch.Close()

	q, err := ch.QueueDeclare(
		queueName, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		nil,       // arguments
	)
	if err != nil {
		panic(err)
	}

	paymentConsumer := service.NewPaymentConsumer(os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT"), os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK"), q, ch)
	go paymentConsumer.StartPaymentConsumer(q, ch)

	handlerPayment := handler.NewPayment(service.NewPayment(queueName, ch))

	testCases := []struct {
		desc     string
		testCase func(t *testing.T)
	}{
		{
			desc: "post",
			testCase: func(t *testing.T) {
				payment := model.Payment{
					CorrelationId: uuid.New().String(),
					Amount:        decimal.New(19, 20),
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
			},
		},
		{
			desc: "post empty body",
			testCase: func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/payments", nil)
				rec := httptest.NewRecorder()

				handlerPayment.Post(rec, req)
				if rec.Code != 400 {
					t.Errorf("expected 400 got %v", rec.Code)
				}
			},
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, tC.testCase)
	}
}

func Test(t *testing.T) {
	testCases := []struct {
		desc string
	}{
		{
			desc: "",
		},
	}
	for _, tC := range testCases {
		t.Run(tC.desc, func(t *testing.T) {

		})
	}
}
