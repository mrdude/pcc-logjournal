package logjournal

import (
	"bytes"
	"slices"
	"testing"
	"time"

	"git.home.prztl.com/git/w/prztl-common-go/pcommon"
)

func TestPartitionIO(t *testing.T) {
	ctx := t.Context()

	logger := pcommon.MustCreateLogger()
	ctx = pcommon.WithLoggerContextValue(ctx, logger)

	d, err := StartJournal(ctx, t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}

	// write some messages
	var now = time.Now()
	var expected = []Entry{
		Entry{
			Ts:   now.Add(-5 * time.Minute).UTC(),
			Data: []byte("hello whirled"),
		},
		Entry{
			Ts:   now.UTC(),
			Data: []byte("goodbyee"),
		},
	}

	err = d.Append(expected)

	if err != nil {
		t.Fatal(err)
	}

	// read the messages back
	scanner := d.ScanJournal(ctx, now.Add(-10*time.Minute), now.Add(10*time.Minute))

	var (
		actual []Entry
		quit   = false
	)
	for !quit {
		select {
		case msg, more := <-scanner.EntryCh:
			if !more {
				quit = true
				break
			}

			actual = append(actual, msg)
		case msg := <-scanner.NoticeCh:
			t.Logf("Notice: %s", msg)
		case err := <-scanner.ErrCh:
			t.Fatal(err)
		}
	}

	slices.SortFunc(actual, func(a, b Entry) int { return a.Ts.Compare(b.Ts) })

	if !slices.EqualFunc(expected, actual, func(a, b Entry) bool {
		return a.Ts.Equal(b.Ts) && bytes.Equal(a.Data, b.Data)
	}) {
		t.Fatal("expected doesn't match actual")
	}
}
