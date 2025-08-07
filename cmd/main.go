package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/lauro-santana/rinha-backend-2025/internal/handler"
	"github.com/lauro-santana/rinha-backend-2025/internal/repository/database"
	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	amqp "github.com/rabbitmq/amqp091-go"
)

var queueName string = "job_queue"

func consumer() {
	var conn *amqp.Connection
	var err error

	for range 30 {
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
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

	db, err := database.NewDatabase(os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("MIGRATE"))
	if err != nil {
		panic(err)
	}
	log.Println("running consumer")
	paymentConsumer := service.NewPaymentConsumer(os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT"), os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK"), q, ch, db)
	paymentConsumer.StartPaymentConsumer()
}

func producer() {
	var conn *amqp.Connection
	var err error

	for range 30 {
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		panic(err)
	}
	ch, err := conn.Channel()
	if err != nil {
		panic(err)
	}
	defer ch.Close()

	db, err := database.NewDatabase(os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("MIGRATE"))
	if err != nil {
		panic(err)
	}

	handlerPayment := handler.NewPayment(service.NewPayment(queueName, ch, db))

	server := http.NewServeMux()

	server.HandleFunc("POST /payments", handlerPayment.Post)
	server.HandleFunc("GET /payments-summary", handlerPayment.Get)
	log.Println("running producer")
	if err = http.ListenAndServe(":"+os.Getenv("SERVER_PORT"), server); err != nil {
		panic(err)
	}
}

func main() {
	flag := os.Getenv("CONSUMER")
	switch flag {
	case "0":
		producer()
	default:
		consumer()
	}
}
