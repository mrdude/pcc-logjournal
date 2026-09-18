package logjournal

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const partitionFilenamePrefix = "pcc-partition-"

// PartitionTimeGranularity is the time period represented by a single dateKey
const PartitionTimeGranularity = 1 * time.Hour

// is used to name a journal file
type PartitionKey struct {
	DateKey
	segment int64 // a segment=-1 is sorted as the last PartitionKey in a (year, yearDay)
}

type DateKey struct {
	year, month, day, hour int64
}

func PartitionKeyFromFilename(name string) (fk PartitionKey, err error) {
	name = filepath.Base(name)

	if !strings.HasPrefix(name, partitionFilenamePrefix+"-") {
		err = fmt.Errorf("invalid filename (no prefix): %s", name)
		return
	}

	name = strings.TrimPrefix(name, partitionFilenamePrefix+"-")

	sp := strings.SplitN(name, "-", 5)
	if len(sp) != 5 {
		err = fmt.Errorf("invalid filename: %s", name)
		return
	}

	fk.year, err = strconv.ParseInt(sp[0], 10, 64)
	if err != nil {
		err = fmt.Errorf("invalid filename (bad parse): %s: %w", name, err)
		return
	}

	fk.month, err = strconv.ParseInt(sp[1], 10, 64)
	if err != nil {
		err = fmt.Errorf("invalid filename (bad parse): %s: %w", name, err)
		return
	}

	fk.day, err = strconv.ParseInt(sp[2], 10, 64)
	if err != nil {
		err = fmt.Errorf("invalid filename (bad parse): %s: %w", name, err)
		return
	}

	fk.hour, err = strconv.ParseInt(sp[3], 10, 64)
	if err != nil {
		err = fmt.Errorf("invalid filename (bad parse): %s: %w", name, err)
		return
	}

	fk.segment, err = strconv.ParseInt(sp[4], 10, 64)
	if err != nil {
		err = fmt.Errorf("invalid filename (bad parse): %s: %w", name, err)
		return
	}

	return
}

func PartitionKeyFromTimestamp(ts time.Time) (fk PartitionKey) {
	fk.year = int64(ts.Year())
	fk.month = int64(ts.Month())
	fk.day = int64(ts.Day())
	fk.hour = int64(ts.Hour())
	fk.segment = 0
	return
}

func (fk PartitionKey) Filename() string {
	return fmt.Sprintf("%s-%04d-%02d-%02d-%02d-%05d",
		partitionFilenamePrefix,
		fk.year,
		fk.month,
		fk.day,
		fk.hour,
		fk.segment)
}

func (fk PartitionKey) Date() time.Time {
	return time.Date(
		int(fk.year),
		time.Month(fk.month),
		int(fk.day),
		int(fk.hour),
		0,
		0,
		0,
		time.UTC,
	)
}

func (fk PartitionKey) String() string {
	return fmt.Sprintf("[%s, %02d hours, seg=%d]", fk.Date().Format(time.DateOnly), fk.hour, fk.segment)
}

func ComparePartitionKeys(fk1, fk2 PartitionKey) int {
	sign := func(i int64) int {
		if i < 0 {
			return -1
		} else if i > 0 {
			return 1
		} else {
			return 0
		}
	}

	if fk1.year != fk2.year {
		return sign(fk1.year - fk2.year)
	}

	if fk1.month != fk2.month {
		return sign(fk1.month - fk2.month)
	}

	if fk1.day != fk2.day {
		return sign(fk1.day - fk2.day)
	}

	if fk1.hour != fk2.hour {
		return sign(fk1.hour - fk2.hour)
	}

	switch {
	case fk1.segment == -1 && fk2.segment == -1:
		return 0
	case fk1.segment != -1 && fk2.segment == -1:
		return 1
	case fk1.segment == -1 && fk2.segment != -1:
		return -1
	default:
		return sign(fk2.segment - fk1.segment)
	}
}
