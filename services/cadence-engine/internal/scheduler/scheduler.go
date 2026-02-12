package scheduler

import (
	"fmt"
	"time"

	"github.com/seidu626/subscription-manager/cadence-engine/internal/domain"
)

const (
	RuleDaily      = "DAILY"
	RuleWeekly     = "WEEKLY"
	RuleEveryNDays = "EVERY_N_DAYS"

	CatchupSend     = "SEND"
	CatchupSkip     = "SKIP"
	CatchupThrottle = "THROTTLE"
)

func FirstSendAt(rule domain.ScheduleRule, now time.Time, anchor time.Time) (time.Time, error) {
	return nextSlot(rule, now, anchor, nil)
}

func NextSendAt(rule domain.ScheduleRule, now time.Time, lastSentAt time.Time, anchor time.Time) (time.Time, error) {
	return nextSlot(rule, now, anchor, &lastSentAt)
}

func nextSlot(rule domain.ScheduleRule, now time.Time, anchor time.Time, lastSentAt *time.Time) (time.Time, error) {
	loc, err := resolveLocation(rule.Timezone)
	if err != nil {
		return time.Time{}, err
	}

	now = now.In(loc)
	anchor = anchor.In(loc)
	var candidate time.Time

	switch rule.RuleKind {
	case RuleDaily:
		candidate = nextDaily(rule, now, lastSentAt)
	case RuleWeekly:
		candidate = nextWeekly(rule, now, lastSentAt)
	case RuleEveryNDays:
		candidate = nextEveryNDays(rule, now, anchor, lastSentAt)
	default:
		return time.Time{}, fmt.Errorf("unsupported rule kind: %s", rule.RuleKind)
	}

	candidate = applySendWindow(candidate, rule, loc)

	if lastSentAt == nil {
		return candidate, nil
	}

	if candidate.Before(now) {
		switch rule.CatchupMode {
		case CatchupSend:
			return candidate, nil
		case CatchupSkip, CatchupThrottle:
			return advanceUntilAfterNow(rule, now, anchor, candidate), nil
		default:
			return advanceUntilAfterNow(rule, now, anchor, candidate), nil
		}
	}

	return candidate, nil
}

func nextDaily(rule domain.ScheduleRule, now time.Time, lastSentAt *time.Time) time.Time {
	preferred := buildPreferredTime(now, rule)
	if lastSentAt == nil {
		if preferred.Before(now) {
			return preferred.AddDate(0, 0, 1)
		}
		return preferred
	}
	return buildPreferredTime(lastSentAt.In(now.Location()).AddDate(0, 0, 1), rule)
}

func nextWeekly(rule domain.ScheduleRule, now time.Time, lastSentAt *time.Time) time.Time {
	base := now
	if lastSentAt != nil {
		base = lastSentAt.In(now.Location()).AddDate(0, 0, 1)
	}

	for i := 0; i < 7; i++ {
		candidate := base.AddDate(0, 0, i)
		if matchesWeekday(candidate, rule.DaysOfWeek) {
			return buildPreferredTime(candidate, rule)
		}
	}

	return buildPreferredTime(base.AddDate(0, 0, 7), rule)
}

func nextEveryNDays(rule domain.ScheduleRule, now time.Time, anchor time.Time, lastSentAt *time.Time) time.Time {
	n := rule.NDays
	if n <= 0 {
		n = 1
	}

	var base time.Time
	if lastSentAt != nil {
		base = lastSentAt.In(now.Location())
	} else {
		base = anchor
	}

	candidate := buildPreferredTime(base, rule)
	for candidate.Before(now) {
		candidate = candidate.AddDate(0, 0, n)
	}
	return candidate
}

func advanceUntilAfterNow(rule domain.ScheduleRule, now time.Time, anchor time.Time, candidate time.Time) time.Time {
	next := candidate
	for next.Before(now) {
		switch rule.RuleKind {
		case RuleDaily:
			next = next.AddDate(0, 0, 1)
		case RuleWeekly:
			next = next.AddDate(0, 0, 7)
		case RuleEveryNDays:
			n := rule.NDays
			if n <= 0 {
				n = 1
			}
			next = next.AddDate(0, 0, n)
		default:
			return applySendWindow(now, rule, now.Location())
		}
		next = applySendWindow(next, rule, next.Location())
	}
	return next
}

func applySendWindow(candidate time.Time, rule domain.ScheduleRule, loc *time.Location) time.Time {
	start := time.Date(candidate.Year(), candidate.Month(), candidate.Day(),
		rule.SendStartTime.Hour(), rule.SendStartTime.Minute(), rule.SendStartTime.Second(), 0, loc)
	end := time.Date(candidate.Year(), candidate.Month(), candidate.Day(),
		rule.SendEndTime.Hour(), rule.SendEndTime.Minute(), rule.SendEndTime.Second(), 0, loc)

	if !end.After(start) {
		return candidate
	}

	if candidate.Before(start) {
		return start
	}
	if candidate.After(end) {
		return start.AddDate(0, 0, 1)
	}
	return candidate
}

func buildPreferredTime(base time.Time, rule domain.ScheduleRule) time.Time {
	return time.Date(base.Year(), base.Month(), base.Day(),
		rule.PreferredTime.Hour(), rule.PreferredTime.Minute(), rule.PreferredTime.Second(), 0, base.Location())
}

func matchesWeekday(candidate time.Time, bitmask int) bool {
	if bitmask == 0 {
		return true
	}

	weekday := candidate.Weekday()
	var bit int
	switch weekday {
	case time.Monday:
		bit = 1
	case time.Tuesday:
		bit = 2
	case time.Wednesday:
		bit = 4
	case time.Thursday:
		bit = 8
	case time.Friday:
		bit = 16
	case time.Saturday:
		bit = 32
	case time.Sunday:
		bit = 64
	}

	return bitmask&bit != 0
}

func resolveLocation(tz string) (*time.Location, error) {
	if tz == "" {
		return time.UTC, nil
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC, fmt.Errorf("invalid timezone %q: %w", tz, err)
	}
	return loc, nil
}
