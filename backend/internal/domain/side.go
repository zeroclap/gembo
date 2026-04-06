package domain

// PositionSide represents the direction of a position leg.
// Exchanges may report various raw values (e.g. "net", "-1"); adapters
// must normalize to one of these constants at the boundary.
type PositionSide string

const (
	PositionSideLong  PositionSide = "long"
	PositionSideShort PositionSide = "short"
	PositionSideNone  PositionSide = "" // position closed, no direction
)

// OrderSide represents the direction of an order.
type OrderSide string

const (
	OrderSideBuy  OrderSide = "buy"
	OrderSideSell OrderSide = "sell"
)
