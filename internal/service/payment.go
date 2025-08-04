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

	"github.com/go-goe/goe"
	"github.com/go-goe/goe/query/where"
	"github.com/lauro-santana/rinha-backend-2025/internal/repository/database"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	None int8 = iota
	Default
	Fallback
)

type Paymenter interface {
	Post(payment model.Payment) error
}

type PaymentSummarer interface {
	Get(from, to time.Time) (*model.PaymentSummary, error)
}

type IPayment interface {
	Paymenter
	PaymentSummarer
}

type payment struct {
	serviceQueue string
	channel      *amqp.Channel
	db           *database.Database
}

func NewPayment(queue string, channel *amqp.Channel, db *database.Database) IPayment {
	return &payment{queue, channel, db}
}

func (s *payment) Post(payment model.Payment) error {
	payment.RequestedAt = time.Now()
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
		log.Println("error on publish payment", err)
		return err
	}
	return nil
}

func (s *payment) Get(from, to time.Time) (*model.PaymentSummary, error) {
	summary := new(model.PaymentSummary)
	for row, err := range goe.Select(s.db.Payment).Where(
		where.And(
			where.GreaterEquals(&s.db.Payment.RequestedAt, from),
			where.LessEquals(&s.db.Payment.RequestedAt, to),
		)).Rows() {
		if err != nil {
			return nil, err
		}

		switch row.OnFallback {
		case true:
			summary.Fallback.TotalRequests++
			summary.Fallback.TotalAmount = summary.Fallback.TotalAmount.Add(row.Amount)
		default:
			summary.Default.TotalRequests++
			summary.Default.TotalAmount = summary.Default.TotalAmount.Add(row.Amount)
		}
	}
	return summary, nil
}

type PaymentConsumer struct {
	defaultHost  string
	fallbackHost string
	channel      *amqp.Channel
	queue        amqp.Queue
	db           *database.Database
}

func NewPaymentConsumer(defaultHost, fallbackHost string, queue amqp.Queue, channel *amqp.Channel, db *database.Database) *PaymentConsumer {
	return &PaymentConsumer{defaultHost, fallbackHost, channel, queue, db}
}

type addr struct {
	sync.Mutex
	flag int8
}

func (a *addr) SetAddr(f int8) {
	a.Lock()
	defer a.Unlock()
	a.flag = f
}

func (a *addr) GetAddr() int8 {
	a.Lock()
	defer a.Unlock()
	return a.flag
}

func (pc *PaymentConsumer) StartPaymentConsumer(queue amqp.Queue, channel *amqp.Channel) {
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
	var addr addr
	go func() {
		for {
			addr.SetAddr(checkHealth(pc.defaultHost, pc.fallbackHost))
			time.Sleep(time.Second * 5)
		}
	}()
	var flag int8
	defaultPayment, fallbackPayment := pc.defaultHost+"/payments", pc.fallbackHost+"/payments"
	for d := range msgs {
		flag = addr.GetAddr()
		switch flag {
		case Default:
			err = postPayment(d.Body, defaultPayment)
			if errors.Is(err, ErrPaymentServer) {
				err = postPayment(d.Body, fallbackPayment)
				addr.SetAddr(Fallback)
			}
		case Fallback:
			err = postPayment(d.Body, fallbackPayment)
		default:
			d.Nack(false, true)
			continue
		}

		if err != nil {
			if errors.Is(err, ErrUnprocessableEntity) {
				d.Ack(false)
				continue
			}
			d.Nack(false, true)
			continue
		}

		var payment model.Payment
		err = json.Unmarshal(d.Body, &payment)
		if err != nil {
			log.Println("error on unmarshal payment", err)
			d.Nack(false, true)
			continue
		}
		payment.OnFallback = flag == Fallback
		err = goe.Insert(pc.db.Payment).One(&payment)
		if err != nil {
			log.Println("error on insert payment", err)
			d.Nack(false, true)
			continue
		}

		d.Ack(false)
	}
}

func checkHealth(defaultHost, fallbackHost string) int8 {
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
		log.Println("using none, default:", defaultHealth, "fallback:", fallbackHealth)
		return None
	}
	if defaultHealth.Failing {
		if fallbackHealth.Failing {
			log.Println("using none, default:", defaultHealth, "fallback:", fallbackHealth)
			return None
		}
		log.Println("using fallback, default is failing", defaultHealth)
		return Fallback
	}
	// TODO: add throughput 100ms as a env
	if defaultHealth.MinResponseTime > 100 {
		log.Println("using fallback throughput", defaultHealth)
		return Fallback
	}
	return Default
}

var ErrUnprocessableEntity error = fmt.Errorf("unprocessable entity")
var ErrPaymentServer error = fmt.Errorf("error on payment server")

func postPayment(paymentBytes []byte, addr string) error {
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
		return ErrPaymentServer
	}
	return nil
}

func getServiceHealth(addr string) (*model.ServiceHealth, error) {
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
