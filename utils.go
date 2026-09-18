package logjournal

import "time"

// IsNotWithinTimeBounds returns true if (from <= entryTs <= to) is *NOT* true.
// TODO write tests for this
func isNotWithinTimeBounds(from, entryTs, to time.Time) bool {
	// time bound filtering:
	//  isWithinBounds  = from <= entry.ts && entry.ts <= to
	//  !isWithinBounds = !(from <= entry.ts && entry.ts <= to)
	//                  = !(from <= entry.ts) || !(entry.ts <= to)
	//                  = (from > entry.ts) || (entry.ts > to)
	return from.After(entryTs) || entryTs.After(to)
}
