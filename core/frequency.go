package core

import (
	"fmt"
	"time"
)

// Frequency defines how often a recurring rule generates occurrences.
type Frequency int

const (
	FrequencyWeekly      Frequency = 1
	FrequencyBiweekly    Frequency = 2
	FrequencySemiMonthly Frequency = 3
	FrequencyMonthly     Frequency = 4
	FrequencyYearly      Frequency = 5
)

func ValidFrequency(f Frequency) bool {
	return f >= FrequencyWeekly && f <= FrequencyYearly
}

// ExpandOccurrences generates all occurrence dates for a recurring rule within [from, to].
// startDate is the anchor for weekly/biweekly calculations.
// dayOfMonth is used for monthly rules (1-28, capped to month end).
// semiMonthlyDays holds two day-of-month values for semi-monthly rules.
func ExpandOccurrences(freq Frequency, startDate, from, to time.Time, dayOfMonth int, semiMonthlyDays [2]int, endDate *time.Time) []time.Time {
	if endDate != nil && from.After(*endDate) {
		return nil
	}
	effectiveTo := to
	if endDate != nil && endDate.Before(to) {
		effectiveTo = *endDate
	}

	switch freq {
	case FrequencyWeekly:
		return expandInterval(startDate, from, effectiveTo, 7)
	case FrequencyBiweekly:
		return expandInterval(startDate, from, effectiveTo, 14)
	case FrequencySemiMonthly:
		return expandSemiMonthly(semiMonthlyDays, from, effectiveTo)
	case FrequencyMonthly:
		return expandMonthly(dayOfMonth, startDate, from, effectiveTo)
	case FrequencyYearly:
		return expandYearly(startDate, from, effectiveTo)
	default:
		return nil
	}
}

// expandInterval handles weekly and biweekly by stepping forward from startDate in dayStep increments.
func expandInterval(startDate, from, to time.Time, dayStep int) []time.Time {
	if startDate.After(to) {
		return nil
	}

	var dates []time.Time
	// If startDate is before from, advance to the first occurrence on or after from.
	cur := startDate
	if cur.Before(from) {
		daysBehind := int(from.Sub(cur).Hours()/24) + 1
		steps := daysBehind / dayStep
		cur = cur.AddDate(0, 0, steps*dayStep)
		if cur.Before(from) {
			cur = cur.AddDate(0, 0, dayStep)
		}
	}
	for !cur.After(to) {
		dates = append(dates, cur)
		cur = cur.AddDate(0, 0, dayStep)
	}
	return dates
}

func expandSemiMonthly(days [2]int, from, to time.Time) []time.Time {
	if days[0] <= 0 || days[1] <= 0 {
		return nil
	}
	d1, d2 := days[0], days[1]
	if d1 > d2 {
		d1, d2 = d2, d1
	}

	var dates []time.Time
	y, m, _ := from.Date()
	loc := from.Location()

	for {
		for _, day := range []int{d1, d2} {
			capped := capDay(y, m, day)
			d := time.Date(y, m, capped, 0, 0, 0, 0, loc)
			if d.Before(from) {
				continue
			}
			if d.After(to) {
				return dates
			}
			dates = append(dates, d)
		}
		m++
		if m > 12 {
			m = 1
			y++
		}
	}
}

func expandMonthly(dayOfMonth int, startDate, from, to time.Time) []time.Time {
	if dayOfMonth <= 0 || dayOfMonth > 31 {
		return nil
	}

	var dates []time.Time
	y, m, _ := startDate.Date()
	loc := startDate.Location()

	// Start from startDate's month, skip forward if needed.
	for {
		capped := capDay(y, m, dayOfMonth)
		d := time.Date(y, m, capped, 0, 0, 0, 0, loc)
		if d.After(to) {
			break
		}
		if !d.Before(from) {
			dates = append(dates, d)
		}
		m++
		if m > 12 {
			m = 1
			y++
		}
	}
	return dates
}

func expandYearly(startDate, from, to time.Time) []time.Time {
	var dates []time.Time
	y := startDate.Year()
	m := startDate.Month()
	day := startDate.Day()
	loc := startDate.Location()

	for {
		capped := capDay(y, m, day)
		d := time.Date(y, m, capped, 0, 0, 0, 0, loc)
		if d.After(to) {
			break
		}
		if !d.Before(from) {
			dates = append(dates, d)
		}
		y++
	}
	return dates
}

// capDay caps a day-of-month to the last day of the given month/year.
func capDay(year int, month time.Month, day int) int {
	lastDay := daysInMonth(year, month)
	if day > lastDay {
		return lastDay
	}
	return day
}

func daysInMonth(year int, month time.Month) int {
	// The zeroth day of the next month is the last day of this month.
	return time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

func (f Frequency) String() string {
	switch f {
	case FrequencyWeekly:
		return "weekly"
	case FrequencyBiweekly:
		return "biweekly"
	case FrequencySemiMonthly:
		return "semi_monthly"
	case FrequencyMonthly:
		return "monthly"
	case FrequencyYearly:
		return "yearly"
	default:
		return fmt.Sprintf("unknown(%d)", int(f))
	}
}
