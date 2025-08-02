package model

import (
	"time"

	"github.com/shopspring/decimal"
)

type Payment struct {
	CorrelationId string          `json:"correlationId" goe:"varchar(36);pk"`
	Amount        decimal.Decimal `json:"amount" goe:"type:decimal(10,2)"`
	RequestedAt   time.Time       `json:"requestedAt"`
	OnFallback    bool            `json:"-"`
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
