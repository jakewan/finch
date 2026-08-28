package core

import "fmt"

// MaxAmountCents bounds the magnitude of any money amount finch stores: 99999999999999
// cents, one cent under $1 trillion.
//
// The bound exists for the projection engine, not for any client. ProjectBalances expands a
// recurring rule into every occurrence in its range and sums them into a running int64
// balance, and GetMonthlyCashFlow accumulates the same amounts per month; neither
// accumulator checks for overflow. A cap this size leaves roughly 92,000 amounts of headroom
// before an int64 (max ~9.223e18) wraps, which covers any occurrence count a realistic
// horizon produces.
//
// It does not make overflow impossible, and must not be read as doing so. Nothing bounds the
// occurrence count: expandInterval steps forward from a caller-supplied start date with no
// cap, and the daemon validates only the format of a projection's date range, not its width.
// A single weekly rule at this cap starting in year 1 exceeds the headroom on its own. The
// cap makes overflow require a deliberately absurd request rather than an ordinary one;
// bounding the horizon itself is a separate question.
//
// A consequence rather than the reason: every stored amount stays below 2^53 cents, so it
// survives a hop through a double-precision number exactly. The Qt app's input field caps at
// this same value, reached from that seam rather than from this one, so client and domain
// agree instead of each carrying its own limit. If the two ever must diverge, this one
// governs.
const MaxAmountCents int64 = 99999999999999

// validateAmountMagnitude enforces the one rule every money-writing path shares. Zero is
// meaningless — it records an event and moves no balance — and the magnitude must stay
// within MaxAmountCents so the projection and cash-flow accumulations keep their overflow
// headroom.
//
// Sign is deliberately free: a non-transfer amount is signed by design, negative for an
// expense and positive for income. The transfer-specific positive-magnitude rule stacks on
// top of this one rather than replacing it, and runs after it.
//
// The bounds are compared directly rather than through an absolute value, because
// math.MinInt64 has no positive counterpart: negating it wraps back to itself, so an
// abs-routed ceiling would admit the one value furthest outside the bound.
func validateAmountMagnitude(amount int64) error {
	if amount == 0 {
		return fmt.Errorf("amount must not be zero: %w", ErrInvalidInput)
	}
	if amount > MaxAmountCents || amount < -MaxAmountCents {
		return fmt.Errorf("amount magnitude must not exceed %d cents, got %d: %w",
			MaxAmountCents, amount, ErrInvalidInput)
	}
	return nil
}
