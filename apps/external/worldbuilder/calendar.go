package worldbuilder

import (
	"math"
	"strconv"
	"strings"
)

// Calendar maps signed fictional ticks to a repeating civil calendar. EpochTick
// begins day one of month one in EpochYear; leap positions are relative to that
// year, repeating forwards and backwards. It is canon, never wall-clock time.
type Calendar struct {
	Name           string          `json:"name"`
	EpochTick      string          `json:"epochTick"`
	EpochYear      string          `json:"epochYear"`
	TicksPerDay    int             `json:"ticksPerDay"`
	Months         []CalendarMonth `json:"months"`
	CycleYears     int             `json:"cycleYears"`
	LeapYears      []int           `json:"leapYears,omitempty"`
	Weekdays       []string        `json:"weekdays,omitempty"`
	WeekdayAtEpoch int             `json:"weekdayAtEpoch"`
}

type CalendarMonth struct {
	Name     string `json:"name"`
	Days     int    `json:"days"`
	LeapDays int    `json:"leapDays,omitempty"`
}

func dateFact(v any) (int64, bool) {
	s, ok := v.(string)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return n, err == nil && strconv.FormatInt(n, 10) == s
}

func lifeTick(r Record) (int64, bool) {
	if r.ValueType == "tick" {
		return dateFact(r.Value)
	}
	// Preserve historical numeric facts; rounded floating-point dates are not
	// silently promoted to full-range dates. New date facts use decimal strings.
	n, ok := r.Value.(float64)
	if r.ValueType != "number" || !ok || math.IsNaN(n) || math.Trunc(n) != n || math.Abs(n) > 9007199254740991 {
		return 0, false
	}
	return int64(n), true
}

func validateCalendar(c *Calendar) error {
	if c == nil {
		return nil
	}
	name := func(s string) bool { return strings.TrimSpace(s) == s && s != "" && len(s) <= 80 }
	_, tickOK := dateFact(c.EpochTick)
	_, yearOK := dateFact(c.EpochYear)
	if !name(c.Name) || !tickOK || !yearOK || c.TicksPerDay < 1 || c.TicksPerDay > 1000000000 || c.CycleYears < 1 || c.CycleYears > 1000 {
		return bad("calendar requires a name (1–80 bytes), exact signed epoch tick/year, 1–1000000000 ticks per day and a 1–1000 year cycle")
	}
	if len(c.Months) < 1 || len(c.Months) > 32 || len(c.Weekdays) > 32 {
		return bad("calendar requires 1–32 months and at most 32 weekdays")
	}
	seen := map[string]bool{}
	days, extra := 0, 0
	for _, m := range c.Months {
		if !name(m.Name) || seen[m.Name] || m.Days < 0 || m.Days > 10000 || m.LeapDays < 0 || m.LeapDays > 10000 || m.Days+m.LeapDays == 0 {
			return bad("calendar month names must be unique; ordinary/leap additions must each be 0–10000 days with a nonempty month")
		}
		seen[m.Name] = true
		days += m.Days
		extra += m.LeapDays
	}
	if days == 0 || (len(c.LeapYears) == 0) != (extra == 0) {
		return bad("calendar needs an ordinary year; leap positions and extra month days must be configured together")
	}
	previous := 0
	for _, year := range c.LeapYears {
		if year <= previous || year > c.CycleYears {
			return bad("calendar leap positions must increase uniquely within 1–cycleYears")
		}
		previous = year
	}
	seen = map[string]bool{}
	for _, day := range c.Weekdays {
		if !name(day) || seen[day] {
			return bad("calendar weekday names must be unique and nonempty (max 80 bytes)")
		}
		seen[day] = true
	}
	if c.WeekdayAtEpoch < 0 || (len(c.Weekdays) == 0 && c.WeekdayAtEpoch != 0) || (len(c.Weekdays) > 0 && c.WeekdayAtEpoch >= len(c.Weekdays)) {
		return bad("calendar epoch weekday must index a configured weekday, or be zero without weekdays")
	}
	return nil
}

func snapshotCalendar(s Snapshot) *Calendar {
	for _, r := range s {
		if r.Kind == "world_settings" {
			return r.Calendar
		}
	}
	return nil
}
