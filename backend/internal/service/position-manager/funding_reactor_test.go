package positionmanager

import (
	"context"
	"testing"
	"time"

	"github.com/shopspring/decimal"

	"github.com/zeroclap/gembo/internal/domain"
)

func TestHandleFundingPaymentEvent_InsertsAndUpdatesPositionFundingPnL(t *testing.T) {
	ps := newMockPositionStore()
	now := time.Now().UTC()

	p := &domain.Position{
		ID:                "pos-funding-event-1",
		UserID:            "test-user-id",
		Symbol:            "ENJ/USDT:USDT",
		Status:            domain.PositionStatusOpen,
		LongExchangeID:    "okx",
		LongCredentialID:  "cred-long",
		ShortExchangeID:   "bybit",
		ShortCredentialID: "cred-short",
		CreatedAt:         now.Add(-8 * time.Hour),
	}
	ps.positions[p.ID] = p

	tr := newTestTrader(ps, &mockTradingExchange{id: "okx"}, &mockTradingExchange{id: "bybit"})

	event := &domain.ExchangeFundingPaymentEvent{
		UserID:     p.UserID,
		ExchangeID: "okx",
		PaymentID:  "okx-bill-1",
		Symbol:     p.Symbol,
		Amount:     decimal.RequireFromString("0.0075"),
		Timestamp:  now,
	}

	if err := tr.handleFundingPaymentEvent(context.Background(), event); err != nil {
		t.Fatalf("handleFundingPaymentEvent error: %v", err)
	}

	payments, err := ps.ListFundingPayments(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("list funding payments: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payment, got %d", len(payments))
	}
	if payments[0].ExternalID != "okx-bill-1" {
		t.Fatalf("expected external id to be persisted, got %q", payments[0].ExternalID)
	}

	updated, err := ps.GetPosition(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("get position: %v", err)
	}
	if updated == nil {
		t.Fatal("expected updated position")
	}
	if !updated.TotalFundingPnL.Equal(decimal.RequireFromString("0.0075")) {
		t.Fatalf("expected total_funding_pnl=0.0075, got %s", updated.TotalFundingPnL)
	}
}

func TestHandleFundingPaymentEvent_DeduplicatesByExternalID(t *testing.T) {
	ps := newMockPositionStore()
	now := time.Now().UTC()

	p := &domain.Position{
		ID:                "pos-funding-event-2",
		UserID:            "test-user-id",
		Symbol:            "ENJ/USDT:USDT",
		Status:            domain.PositionStatusOpen,
		LongExchangeID:    "okx",
		LongCredentialID:  "cred-long",
		ShortExchangeID:   "bybit",
		ShortCredentialID: "cred-short",
		CreatedAt:         now.Add(-8 * time.Hour),
	}
	ps.positions[p.ID] = p

	tr := newTestTrader(ps, &mockTradingExchange{id: "okx"}, &mockTradingExchange{id: "bybit"})

	event := &domain.ExchangeFundingPaymentEvent{
		UserID:     p.UserID,
		ExchangeID: "okx",
		PaymentID:  "okx-bill-dup",
		Symbol:     p.Symbol,
		Amount:     decimal.RequireFromString("0.0100"),
		Timestamp:  now,
	}

	if err := tr.handleFundingPaymentEvent(context.Background(), event); err != nil {
		t.Fatalf("first handleFundingPaymentEvent error: %v", err)
	}
	if err := tr.handleFundingPaymentEvent(context.Background(), event); err != nil {
		t.Fatalf("second handleFundingPaymentEvent error: %v", err)
	}

	payments, err := ps.ListFundingPayments(context.Background(), p.UserID, p.ID)
	if err != nil {
		t.Fatalf("list funding payments: %v", err)
	}
	if len(payments) != 1 {
		t.Fatalf("expected 1 payment after duplicate event, got %d", len(payments))
	}
}
