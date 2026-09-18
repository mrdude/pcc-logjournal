package logjournal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"sync"
	"time"

	"git.home.prztl.com/git/w/prztl-common-go/pcommon"
	"go.uber.org/zap"
)

// Journal represents a logical append-only list of Entry's,
// split into multiple physical files in a single directory.
type Journal struct {
	journalDir *dir

	writeMu sync.Mutex // protects writers and all indexWriter's within
	writers map[PartitionKey]*indexWriter
}

type Entry struct {
	Ts   time.Time
	Data []byte
}

// StartJournal opens a journal for appending
func StartJournal(ctx context.Context, journalDir string) (*Journal, error) {
	if err := os.Mkdir(journalDir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("failed to create journal dir: %w", err)
	}

	j := &Journal{
		journalDir: &dir{d: journalDir},
		writers:    make(map[PartitionKey]*indexWriter),
	}

	// start the garbage collector
	go func() {
		logger := pcommon.GetLogger(ctx).Named("journal-garbage-collector")
		ctx = pcommon.WithLoggerContextValue(ctx, logger)

		logger.Info("starting garbage collector")

		t := time.NewTicker(5 * time.Second)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				j.gc(ctx)
				t.Reset(PartitionTimeGranularity)
			}
		}
	}()

	return j, nil
}

func (j *Journal) gc(ctx context.Context) {
	logger := pcommon.GetLogger(ctx)

	logger.Info("executing GC")
	deleted, err := executeGC(ctx, j.journalDir)
	if err != nil {
		logger.Error("Failed to execute GC", zap.Error(err))
		return
	}

	j.writeMu.Lock()
	defer j.writeMu.Unlock()

	for _, fk := range deleted {
		base := fk
		base.segment = 0

		iw := j.writers[base]
		if iw == nil {
			continue
		}

		if iw.currentKey == fk {
			logger.Info("closing garbage collected index writer",
				zap.String("key", fk.Filename()),
			)
			iw.Close()
			delete(j.writers, base)
		}
	}

	logger.Info("GC completed")
}

// assumes j.writeMu is held
func (j *Journal) getIndexWriter(key PartitionKey) (*indexWriter, error) {
	key.segment = 0

	iw, ok := j.writers[key]
	if ok {
		return iw, nil
	}

	var err error
	iw, err = newIndexWriter(j.journalDir, key)
	if err == nil {
		// FIXME: need a way to close writers when the append window closes
		j.writers[key] = iw
	}
	return iw, err
}

// Append adds entries to the journal
// This method is thread-safe
// TODO add tests; a bug in this method would mean that entries get appended to the wrong indexWriter and never get searched
func (j *Journal) Append(entries []Entry) error {
	// sort the entries
	slices.SortFunc(entries, func(a, b Entry) int {
		return a.Ts.Compare(b.Ts)
	})

	// grab the lock
	j.writeMu.Lock()
	defer j.writeMu.Unlock()

	// scan the journal entries, and append them to the correct indexWriter
	findEndOfChunk := func(fk PartitionKey, start int) (endExclusive int) {
		endExclusive = start
		for ; endExclusive < len(entries); endExclusive++ {
			if PartitionKeyFromTimestamp(entries[endExclusive].Ts).Date() != fk.Date() {
				return
			}
		}

		return
	}

	for i := 0; i < len(entries); i++ {
		fk := PartitionKeyFromTimestamp(entries[i].Ts)

		// find the end of the chunk
		endExclusive := findEndOfChunk(fk, i)

		// all the entries in this chunk are
		// assigned to the same indexWriter
		//
		// append the chunk
		iw, err := j.getIndexWriter(fk)
		if err != nil {
			return err
		}

		err = iw.Append(entries[i:endExclusive])
		if err != nil {
			return err
		}

		// advance
		i = endExclusive
	}

	return nil
}
