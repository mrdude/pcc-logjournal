package logjournal

import (
	"fmt"
	"testing"
	"time"
)

func TestPartitionKeyFromFilename(t *testing.T) {
	type testCase struct {
		filename string
		pk       PartitionKey
		err      error
	}

	cases := []testCase{
		{
			filename: partitionFilenamePrefix + "-2006-01-02-04-00001",
			pk: PartitionKey{
				DateKey: DateKey{
					year:  2006,
					month: 1,
					day:   2,
					hour:  4,
				},
				segment: 1,
			},
		},
		// TODO more test cases
	}
	for i := range cases {
		tc := &cases[i]
		name := tc.filename

		t.Run(name, func(t *testing.T) {
			actual, err := PartitionKeyFromFilename(tc.filename)
			if err != nil && tc.err == nil {
				t.Fatalf("Expected no error, got %s", err)
			} else if actual != tc.pk {
				t.Fatalf("PartitionKeyFromFilename(%s) = %s (expected %s)",
					tc.filename,
					actual.String(), tc.pk.String(),
				)
			}
		})
	}
}

func FuzzPartitionKeyFromFilename(f *testing.F) {
	// TODO seed corpus w/more cases
	f.Add(partitionFilenamePrefix + "2025-12-01-01-00001")

	f.Fuzz(func(t *testing.T, filename string) {
		pk, err := PartitionKeyFromFilename(filename)
		t.Logf("filename = %s, pk = %s, err = %s", filename, pk.String(), err)
	})
}

func TestPartitionKeyFromTimestamp(t *testing.T) {
	type testCase struct {
		time     time.Time
		expected PartitionKey
	}

	cases := []testCase{
		{
			time: time.Date(2025, time.December, 1, 0, 0, 0, 0, time.UTC),
			expected: PartitionKey{
				DateKey: DateKey{
					year:  2025,
					month: 12,
					day:   1,
					hour:  0,
				},
				segment: 0,
			},
		},
		// TODO more test cases
	}
	for i := range cases {
		tc := &cases[i]
		name := tc.time.Format(time.DateTime)

		t.Run(name, func(t *testing.T) {
			actual := PartitionKeyFromTimestamp(tc.time)
			if actual != tc.expected {
				t.Fatalf("PartitionKeyFromTimestamp(%s) = %s (expected %s)",
					tc.time.Format(time.DateTime),
					actual.String(), tc.expected.String(),
				)
			}
		})
	}
}

func FuzzPartitionKeyFromTimestamp(f *testing.F) {
	// seed corpus
	for i := range 10 {
		f.Add(int64(i * 1000))
	}

	f.Fuzz(func(t *testing.T, tsint int64) {
		ts := time.UnixMicro(tsint)
		pk := PartitionKeyFromTimestamp(ts)

		if !(int(pk.year) == ts.Year() &&
			int(pk.month) == int(ts.Month()) &&
			int(pk.day) == ts.Day() &&
			int(pk.hour) == ts.Hour() &&
			pk.segment == 0) {
			t.Fatalf("Failed (tsint = %d, time = %s, pk = %s)", tsint, ts.Format(time.DateTime), pk.String())
		}
	})
}

func TestComparePartitionKeys(t *testing.T) {
	type testCase struct {
		fk1, fk2 PartitionKey
		expected int
	}

	cases := []testCase{
		//"start":"[2024-12-16, seg=0]","fk":"[2024-12-16, seg=6]","end":"[2024-12-16, seg=-1]"}
		{
			fk1:      fkDate(2024, time.December, 16, 0, 0),
			fk2:      fkDate(2024, time.December, 16, 0, 6),
			expected: 1,
		},
		{
			fk1:      fkDate(2024, time.December, 16, 0, 6),
			fk2:      fkDate(2024, time.December, 16, 0, -1),
			expected: 1,
		},

		{
			fk1:      fkDate(2024, time.December, 16, 0, 0),
			fk2:      fkDate(2024, time.December, 9, 0, 50),
			expected: 1,
		},

		{
			fk1:      fkDate(2022, time.December, 16, 0, 0),
			fk2:      fkDate(2024, time.December, 9, 0, 50),
			expected: -1,
		},

		{
			fk1:      fkDate(2024, time.December, 9, 0, 0),
			fk2:      fkDate(2024, time.December, 9, 0, 0),
			expected: 0,
		},
	}
	for i := range cases {
		tc := &cases[i]
		name := fmt.Sprintf("cmp(%s, %s)", tc.fk1.String(), tc.fk2.String())
		t.Run(name, func(t *testing.T) {
			actual := ComparePartitionKeys(tc.fk1, tc.fk2)
			if actual != tc.expected {
				t.Fatalf("cmp(%s, %s) = %d (expected %d)",
					tc.fk1.Filename(), tc.fk2.Filename(),
					actual, tc.expected,
				)
			}
		})
	}
}

func fkDate(year int, month time.Month, day, hour int, segment int64) PartitionKey {
	t := time.Date(year, month, day, hour, 0, 0, 0, time.UTC)
	key := PartitionKeyFromTimestamp(t)
	key.segment = segment
	return key
}
