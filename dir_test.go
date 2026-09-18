package logjournal

import (
	"fmt"
	"slices"
	"testing"
	"time"
)

func TestNextKeyForDate(t *testing.T) {
	pk := func(year, month, day, hour, segment int64) PartitionKey {
		return PartitionKey{
			DateKey: DateKey{
				year:  year,
				month: month,
				day:   day,
				hour:  hour,
			},
			segment: segment,
		}
	}

	type TestCase struct {
		Current PartitionKey
		Keys    []PartitionKey

		Expected PartitionKey
	}

	cases := []TestCase{
		// base case: when there are no keys, return Current
		{
			Current: pk(2025, int64(time.January), 1, 1, 0),
			Keys:    []PartitionKey{},

			Expected: pk(2025, int64(time.January), 1, 1, 0),
		},

		{
			Current: pk(2025, int64(time.January), 1, 1, 0),
			Keys: []PartitionKey{
				pk(2025, int64(time.January), 1, 1, 0),
				pk(2025, int64(time.January), 1, 1, 1),
				pk(2025, int64(time.January), 1, 2, 0),
			},

			Expected: pk(2025, int64(time.January), 1, 1, 2),
		},

		{
			Current: pk(2025, int64(time.January), 1, 5, 0),
			Keys: []PartitionKey{
				pk(2025, int64(time.January), 1, 1, 0),
				pk(2025, int64(time.January), 1, 2, 0),
				pk(2025, int64(time.January), 1, 3, 0),
				pk(2025, int64(time.January), 1, 4, 0),
				pk(2025, int64(time.January), 1, 5, 0),
			},

			Expected: pk(2025, int64(time.January), 1, 5, 1),
		},
	}

	for i := range cases {
		tc := &cases[i]
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			slices.SortFunc(tc.Keys, ComparePartitionKeys)
			actual := nextKeyForDate(tc.Current, tc.Keys)
			if actual != tc.Expected {
				t.Fatalf("Mismatch (keys = %q, current = %s, actual = %s, expected = %s)",
					tc.Keys, tc.Current, actual, tc.Expected)
			}
		})
	}
}
