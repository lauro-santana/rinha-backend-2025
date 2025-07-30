package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Payment struct {
	CorrelationId string          `json:"correlationId"`
	Amount        decimal.Decimal `json:"amount"`
}

type Summary struct {
	TotalRequests uint            `json:"totalRequests"`
	TotalAmount   decimal.Decimal `json:"totalAmount"`
}

type PaymentSummary struct {
	Default  Summary `json:"default"`
	Fallback Summary `json:"fallback"`
}

type ServiceHealth struct {
	Failing         bool `json:"failing"`
	MinResponseTime uint `json:"minResponseTime"`
}

type PaymentPost struct {
	Payment
	RequestedAt time.Time `json:"requestedAt"`
}
