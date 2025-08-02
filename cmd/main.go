package main

import (
	"net/http"
	"os"
	"time"

	"github.com/lauro-santana/rinha-backend-2025/internal/handler"
	"github.com/lauro-santana/rinha-backend-2025/internal/repository/database"
	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	amqp "github.com/rabbitmq/amqp091-go"
)

func main() {
	queueName := "job_queue"
	var conn *amqp.Connection
	var err error

	for range 10 {
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			break
		}
		time.Sleep(3 * time.Second)
	}

	if err != nil {
		panic(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer ch.Close()

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

	db, err := database.NewDatabase()
	if err != nil {
		panic(err)
	}

	paymentConsumer := service.NewPaymentConsumer(os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT"), os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK"), q, ch, db)
	go paymentConsumer.StartPaymentConsumer(q, ch)

	addr := os.Getenv("HOST") + ":" + os.Getenv("PORT")

	handlerPayment := handler.NewPayment(service.NewPayment(queueName, ch, db))

	server := http.NewServeMux()

	server.HandleFunc("POST /payments", handlerPayment.Post)
	server.HandleFunc("GET /payments-summary", handlerPayment.Get)

	if err = http.ListenAndServe(addr, server); err != nil {
		panic(err)
	}
}
