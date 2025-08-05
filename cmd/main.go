package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/golang-queue/queue"
	"github.com/golang-queue/queue/core"
	"github.com/lauro-santana/rinha-backend-2025/internal/handler"
	"github.com/lauro-santana/rinha-backend-2025/internal/repository/database"
	"github.com/lauro-santana/rinha-backend-2025/internal/service"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
)

func main() {
	db, err := database.NewDatabase(os.Getenv("POSTGRES_HOST"), os.Getenv("POSTGRES_PORT"), os.Getenv("MIGRATE"))
	if err != nil {
		panic(err)
	}

	pool_size, err := strconv.Atoi(os.Getenv("POOL_SIZE"))
	if err != nil {
		panic(err)
	}
	channel_buffer, err := strconv.Atoi(os.Getenv("CHANNEL_BUFFER"))
	if err != nil {
		panic(err)
	}

	channel := make(chan model.Payment, channel_buffer)
	pool := queue.NewPool(int64(pool_size), queue.WithFn(func(ctx context.Context, m core.TaskMessage) error {
		var v model.Payment
		if err := json.Unmarshal(m.Payload(), &v); err != nil {
			return err
		}

		channel <- v
		return nil
	}))

	channel_consumer, err := strconv.Atoi(os.Getenv("CHANNEL_CONSUMER"))
	if err != nil {
		panic(err)
	}

	paymentConsumer := service.NewPaymentConsumer(os.Getenv("PAYMENT_PROCESSOR_URL_DEFAULT"), os.Getenv("PAYMENT_PROCESSOR_URL_FALLBACK"), db, pool, channel)
	go paymentConsumer.StartPaymentConsumer(channel_consumer)

	handlerPayment := handler.NewPayment(service.NewPayment(db, pool))

	server := http.NewServeMux()

	server.HandleFunc("POST /payments", handlerPayment.Post)
	server.HandleFunc("GET /payments-summary", handlerPayment.Get)
	log.Println("running app")
	if err = http.ListenAndServe(":"+os.Getenv("SERVER_PORT"), server); err != nil {
		panic(err)
	}
}
