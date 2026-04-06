package ccxtutil

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func ptrFloat(f float64) *float64 { return &f }
func ptrString(s string) *string  { return &s }
func ptrBool(b bool) *bool        { return &b }

func TestFloatToDecimal(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got := FloatToDecimal(nil)
		if !got.Equal(decimal.Zero) {
			t.Errorf("FloatToDecimal(nil) = %s, want 0", got)
		}
	})
	t.Run("value", func(t *testing.T) {
		got := FloatToDecimal(ptrFloat(42.5))
		if !got.Equal(decimal.NewFromFloat(42.5)) {
			t.Errorf("FloatToDecimal(42.5) = %s, want 42.5", got)
		}
	})
	t.Run("zero", func(t *testing.T) {
		got := FloatToDecimal(ptrFloat(0))
		if !got.Equal(decimal.Zero) {
			t.Errorf("FloatToDecimal(0) = %s, want 0", got)
		}
	})
}

func TestDerefString(t *testing.T) {
	if got := DerefString(nil); got != "" {
		t.Errorf("DerefString(nil) = %q, want empty", got)
	}
	if got := DerefString(ptrString("hello")); got != "hello" {
		t.Errorf("DerefString(hello) = %q, want hello", got)
	}
}

func TestDerefBool(t *testing.T) {
	if got := DerefBool(nil); got {
		t.Error("DerefBool(nil) = true, want false")
	}
	if got := DerefBool(ptrBool(true)); !got {
		t.Error("DerefBool(true) = false, want true")
	}
}

func TestTimestampToTime(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		if !TimestampToTime(nil).IsZero() {
			t.Error("expected zero time for nil input")
		}
	})
	t.Run("zero", func(t *testing.T) {
		zero := float64(0)
		if !TimestampToTime(&zero).IsZero() {
			t.Error("expected zero time for zero input")
		}
	})
	t.Run("valid", func(t *testing.T) {
		ts := float64(1700000000000)
		got := TimestampToTime(&ts)
		if got.IsZero() {
			t.Error("expected non-zero time")
		}
		if got.Unix() != 1700000000 {
			t.Errorf("Unix() = %d, want 1700000000", got.Unix())
		}
	})
	t.Run("subsecond", func(t *testing.T) {
		ts := float64(1700000000500) // 500ms
		got := TimestampToTime(&ts)
		if got.UnixMilli() != 1700000000500 {
			t.Errorf("UnixMilli() = %d, want 1700000000500", got.UnixMilli())
		}
	})
}

func TestIntervalFromString(t *testing.T) {
	tests := []struct {
		input *string
		want  int
	}{
		{nil, 0},
		{ptrString("1h"), 1},
		{ptrString("2h"), 2},
		{ptrString("4h"), 4},
		{ptrString("8h"), 8},
		{ptrString("12h"), 12},
		{ptrString("24h"), 24},
		{ptrString("unknown"), 0},
		{ptrString(""), 0},
	}

	for _, tt := range tests {
		name := "nil"
		if tt.input != nil {
			name = *tt.input
			if name == "" {
				name = "empty"
			}
		}
		t.Run(name, func(t *testing.T) {
			got := IntervalFromString(tt.input)
			if got != tt.want {
				t.Errorf("IntervalFromString(%v) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestNextBackoff(t *testing.T) {
	t.Run("grows", func(t *testing.T) {
		first := NextBackoff(1*time.Second, MaxBackoff)
		if first <= 1*time.Second {
			t.Errorf("expected backoff > 1s, got %s", first)
		}
	})
	t.Run("caps at max", func(t *testing.T) {
		got := NextBackoff(50*time.Second, MaxBackoff)
		if got != MaxBackoff {
			t.Errorf("expected %s, got %s", MaxBackoff, got)
		}
	})
	t.Run("already at max", func(t *testing.T) {
		got := NextBackoff(MaxBackoff, MaxBackoff)
		if got != MaxBackoff {
			t.Errorf("expected %s, got %s", MaxBackoff, got)
		}
	})
}
