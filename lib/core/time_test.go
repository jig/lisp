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
