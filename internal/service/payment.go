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
	"github.com/golang-queue/queue"
	"github.com/lauro-santana/rinha-backend-2025/internal/repository/database"
	"github.com/lauro-santana/rinha-backend-2025/pkg/model"
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
	db   *database.Database
	pool *queue.Queue
}

func NewPayment(db *database.Database, pool *queue.Queue) IPayment {
	return &payment{db, pool}
}

func (s *payment) Post(payment model.Payment) error {
	err := s.pool.Queue(payment)
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
	db           *database.Database
	pool         *queue.Queue
	channel      <-chan model.Payment
}

func NewPaymentConsumer(defaultHost, fallbackHost string, db *database.Database, pool *queue.Queue, channel <-chan model.Payment) *PaymentConsumer {
	return &PaymentConsumer{defaultHost, fallbackHost, db, pool, channel}
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

func (pc *PaymentConsumer) StartPaymentConsumer(channel_consumer int) {
	var addr addr
	go func() {
		for {
			addr.SetAddr(checkHealth(pc.defaultHost, pc.fallbackHost))
			time.Sleep(time.Second * 5)
		}
	}()
	defaultPayment, fallbackPayment := pc.defaultHost+"/payments", pc.fallbackHost+"/payments"
	var wg sync.WaitGroup
	for range channel_consumer {
		wg.Add(1)
		go func() {
			pc.startWorker(defaultPayment, fallbackPayment, &addr, &wg)
		}()
	}
	wg.Wait()
}

func (pc *PaymentConsumer) startWorker(defaultPayment, fallbackPayment string, addr *addr, wg *sync.WaitGroup) {
	var err error
	var flag int8

	for payment := range pc.channel {
		flag = addr.GetAddr()
		switch flag {
		case Default:
			err = postPayment(defaultPayment, &payment)
			if errors.Is(err, ErrPaymentServer) {
				err = postPayment(fallbackPayment, &payment)
				addr.SetAddr(Fallback)
			}
		case Fallback:
			err = postPayment(fallbackPayment, &payment)
		default:
			pc.pool.Queue(payment)
			continue
		}

		if err != nil {
			if errors.Is(err, ErrUnprocessableEntity) {
				continue
			}
			pc.pool.Queue(payment)
			continue
		}

		payment.OnFallback = flag == Fallback
		err = goe.Insert(pc.db.Payment).One(&payment)
		if err != nil {
			log.Println("error on insert payment", err)
			pc.pool.Queue(payment)
			continue
		}
	}
	wg.Done()
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

func postPayment(addr string, p *model.Payment) error {
	p.RequestedAt = time.Now()
	paymentBytes, err := json.Marshal(p)
	if err != nil {
		log.Println("error on marshal payment", err)
		return err
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
