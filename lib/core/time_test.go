package core_test

import (
	"testing"
)

// time-format / time-parse convert between the epoch milliseconds of
// time-ms and RFC 3339 UTC timestamps.
func TestTimeFormatParse(t *testing.T) {
	ns := newEnv(t)

	for _, tc := range []struct{ src, want string }{
		// fixed millisecond precision, always UTC
		{`(time-format 0)`, `"1970-01-01T00:00:00.000Z"`},
		{`(time-format 1234567890123)`, `"2009-02-13T23:31:30.123Z"`},
		{`(time-format -1000)`, `"1969-12-31T23:59:59.000Z"`},
		// parse accepts optional fractional seconds and zone offsets
		{`(time-parse "1970-01-01T00:00:00Z")`, `0`},
		{`(time-parse "2009-02-13T23:31:30.123Z")`, `1234567890123`},
		{`(time-parse "2009-02-14T01:31:30+02:00")`, `1234567890000`},
		// round-trip on the current clock
		{`(let [ms (time-ms)] (= ms (time-parse (time-format ms))))`, `true`},
		// a malformed timestamp is a catchable error
		{`(try (time-parse "not a timestamp") (catch e :bad))`, `:bad`},
	} {
		if got := run(t, ns, tc.src); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.src, got, tc.want)
		}
	}
}

// time-add shifts epoch milliseconds by a hash-map of deltas; :years
// :months :days are calendar-aware (via UTC), the rest are fixed durations.
func TestTimeAdd(t *testing.T) {
	ns := newEnv(t)

	for _, tc := range []struct{ src, want string }{
		// fixed durations add up
		{`(time-format (time-add 0 {:hours 1 :minutes 30}))`, `"1970-01-01T01:30:00.000Z"`},
		{`(time-format (time-add 0 {:milliseconds 123}))`, `"1970-01-01T00:00:00.123Z"`},
		// calendar arithmetic normalises overflow like Go's AddDate:
		// 2021 is not a leap year, so Feb 29 + 1 year rolls to March 1,
		// and Jan 31 + 1 month (no Feb 31) rolls to March 2.
		{`(time-format (time-add (time-parse "2020-02-29T12:00:00Z") {:years 1}))`, `"2021-03-01T12:00:00.000Z"`},
		{`(time-format (time-add (time-parse "2020-01-31T00:00:00Z") {:months 1}))`, `"2020-03-02T00:00:00.000Z"`},
		// negative deltas shift backwards
		{`(time-format (time-add 0 {:seconds -1}))`, `"1969-12-31T23:59:59.000Z"`},
		// an empty map is a no-op
		{`(time-add 12345 {})`, `12345`},
		// unknown key and non-integer value are catchable errors
		{`(try (time-add 0 {:fortnights 2}) (catch e :bad))`, `:bad`},
		{`(try (time-add 0 {:hours "2"}) (catch e :bad))`, `:bad`},
	} {
		if got := run(t, ns, tc.src); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.src, got, tc.want)
		}
	}
}

func TestTimeBeforeAfter(t *testing.T) {
	ns := newEnv(t)

	for _, tc := range []struct{ src, want string }{
		{`(time-before? 1 2)`, `true`},
		{`(time-before? 2 1)`, `false`},
		{`(time-before? 5 5)`, `false`},
		{`(time-after? 2 1)`, `true`},
		{`(time-after? 1 2)`, `false`},
		{`(time-after? 5 5)`, `false`},
	} {
		if got := run(t, ns, tc.src); got != tc.want {
			t.Errorf("%s: got %s, want %s", tc.src, got, tc.want)
		}
	}
}
