package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
	amqp "github.com/rabbitmq/amqp091-go"
)

type Paymenter interface {
	Post(payment model.Payment) error
}

type payment struct {
	serviceQueue string
	channel      *amqp.Channel
}

func NewPayment(queue string, channel *amqp.Channel) Paymenter {
	return &payment{queue, channel}
}

func (s *payment) Post(payment model.Payment) error {
	jsonBytes, err := json.Marshal(payment)
	if err != nil {
		log.Println("error on json marshal payment", err)
		return err
	}
	err = s.channel.Publish(
		"",             // exchange
		s.serviceQueue, // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(jsonBytes),
		},
	)
	if err != nil {
		log.Println("error on json marshal payment", err)
		return err
	}
	return nil
}

type PaymentConsumer struct {
	defaultHost  string
	fallbackHost string
	channel      *amqp.Channel
	queue        amqp.Queue
}

func NewPaymentConsumer(defaultHost, fallbackHost string, queue amqp.Queue, channel *amqp.Channel) *PaymentConsumer {
	return &PaymentConsumer{defaultHost, fallbackHost, channel, queue}
}

func (pc *PaymentConsumer) StartPaymentConsumer(queue amqp.Queue, channel *amqp.Channel) {
	log.Println("starting consumer")
	msgs, err := channel.Consume(
		queue.Name, // queue
		"",         // consumer
		false,      // auto-ack
		false,      // exclusive
		false,      // no-local
		false,      // no-wait
		nil,        // args
	)
	if err != nil {
		log.Fatal(err)
	}
	var addr string
	go func() {
		log.Println("start checking services health")
		for {
			addr = checkHealth(pc.defaultHost, pc.fallbackHost)
			time.Sleep(time.Second * 5)
		}
	}()

	for d := range msgs {
		if err = postPayment(d.Body, addr); err != nil && !errors.Is(err, ErrUnprocessableEntity) {
			d.Nack(false, true)
			continue
		}
		d.Ack(false)
		// TODO: add database
	}
	log.Println("ending consumer")
}

func checkHealth(defaultHost, fallbackHost string) string {
	var wg sync.WaitGroup
	var defaultHealth *model.ServiceHealth
	var fallbackHealth *model.ServiceHealth
	wg.Add(1)
	go func() {
		defaultHealth, _ = getServiceHealth(defaultHost + "/payments/service-health")
		wg.Done()
	}()
	wg.Add(1)
	go func() {
		fallbackHealth, _ = getServiceHealth(fallbackHost + "/payments/service-health")
		wg.Done()
	}()
	wg.Wait()

	if defaultHealth == nil && fallbackHealth == nil {
		log.Println("both services are not avaliable")
		return ""
	}
	if defaultHealth.Failing {
		if fallbackHealth.Failing {
			log.Println("both services are not avaliable")
			return ""
		}
		log.Println("default failing! using fallback host")
		return fallbackHost + "/payments"
	}
	// TODO: add throughput 100ms as a env
	if defaultHealth.MinResponseTime > 100 {
		log.Println("throughput! using fallback host")
		return fallbackHost + "/payments"
	}
	log.Println("using default host")
	return defaultHost + "/payments"
}

var ErrUnprocessableEntity error = fmt.Errorf("unprocessable entity")

func postPayment(paymentBytes []byte, addr string) error {
	if addr == "" {
		log.Println("empty addr")
		return fmt.Errorf("empty addr")
	}
	req, err := http.NewRequest(http.MethodPost, addr, bytes.NewBuffer(paymentBytes))
	if err != nil {
		log.Printf("error %v on new post request to %v\n", err, addr)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error %v on post request to %v\n", err, addr)
		return err
	}
	defer res.Body.Close()
	if res.StatusCode == 422 {
		return ErrUnprocessableEntity
	}
	if res.StatusCode != 200 {
		log.Printf("status code %v on %v\n", res.StatusCode, addr)
	}
	return nil
}

func getServiceHealth(addr string) (*model.ServiceHealth, error) {
	log.Println("request service health to", addr)
	req, err := http.NewRequest(http.MethodGet, addr, nil)
	if err != nil {
		log.Printf("error %v on new get request to %v\n", err, addr)
		return nil, err
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("error %v on get request to %v\n", err, addr)
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode == 429 {
		return nil, fmt.Errorf("429 from server : Too Many Requests")
	}
	var health model.ServiceHealth
	err = json.NewDecoder(res.Body).Decode(&health)
	if err != nil {
		log.Printf("error %v on decode response from %v\n", err, addr)
		return nil, err
	}
	return &health, nil
}
