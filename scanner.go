package logjournal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"git.home.prztl.com/git/w/prztl-common-go/pcommon"
	"go.uber.org/zap"
)

const (
	kb        = 1024
	mb        = 1024 * kb
	gb        = 1024 * mb
	chunkSize = 512 * mb
)

// Scanner scans a journal and returns all Entry's
// within the given time span
type Scanner struct {
	EntryCh  <-chan Entry
	ErrCh    <-chan error
	NoticeCh <-chan string

	entryCh  chan<- Entry
	errCh    chan<- error
	noticeCh chan<- string

	journalDir         *dir
	startTime, endTime time.Time

	totalBytesProcessed, nextProcessedNotice int64
	totalBytesScanned, nextScanNotice        int64
}

func (j *Journal) ScanJournal(ctx context.Context, startTime, endTime time.Time) *Scanner {
	logger := pcommon.GetLogger(ctx).Named("journal-scan")
	ctx = pcommon.WithLoggerContextValue(ctx, logger)

	var (
		entryCh  = make(chan Entry)
		errCh    = make(chan error)
		noticeCh = make(chan string)
	)

	s := &Scanner{
		EntryCh:  entryCh,
		ErrCh:    errCh,
		NoticeCh: noticeCh,

		entryCh:  entryCh,
		errCh:    errCh,
		noticeCh: noticeCh,

		journalDir: j.journalDir,
		startTime:  startTime,
		endTime:    endTime,
	}
	s.start(ctx)
	return s
}

func (s *Scanner) start(ctx context.Context) {
	var (
		entryCh    = s.entryCh
		errCh      = s.errCh
		noticeCh   = s.noticeCh
		journalDir = s.journalDir
		startTime  = s.startTime
		endTime    = s.endTime
	)

	go func() {
		logger := pcommon.GetLogger(ctx)

		// before returning, close the channels to signal the end of the stream
		defer close(entryCh)
		defer close(errCh)
		defer close(noticeCh)

		// scan the journal dir
		filekeys, err := journalDir.ListFileKeys()
		if err != nil {
			errCh <- err
			return
		}

		// determine the filekeys to scan
		// TODO use a binary search?
		var keysToScan []PartitionKey
		var keyFilenames []string
		startScanFk := PartitionKeyFromTimestamp(startTime)
		endScanFk := PartitionKeyFromTimestamp(endTime)
		endScanFk.segment = -1

		for _, fk := range filekeys {
			if ComparePartitionKeys(startScanFk, fk) <= 0 && ComparePartitionKeys(fk, endScanFk) >= 0 {
				keysToScan = append(keysToScan, fk)
				keyFilenames = append(keyFilenames, fk.Filename())
			}
		}
		logger.Info("keys to scan",
			zap.Strings("keyFilenames", keyFilenames))

		// scan the filekeys
		// TODO scan in parallel
		for _, fk := range keysToScan {
			if err := ctx.Err(); err != nil {
				return
			}

			logger.Info("scanning file",
				zap.String("filename", fk.Filename()))
			s.scanFile(ctx, fk)
		}

		s.pushByteNotice()
	}()
}

func (s *Scanner) scanFile(ctx context.Context, fk PartitionKey) {
	var (
		journalDir = s.journalDir
		startTime  = s.startTime
		endTime    = s.endTime
		entryCh    = s.entryCh
		errCh      = s.errCh
	)

	// open the journal file
	filename := filepath.Join(journalDir.Dir(), fk.Filename())
	fd, err := os.Open(filename)
	if err != nil {
		errCh <- fmt.Errorf("failed to read '%s': %w", fk.Filename(), err)
		return
	}
	defer fd.Close()

	// determine the journal file version
	fileVersion, err := detectJournalFileVersion(fd)
	if err != nil {
		errCh <- fmt.Errorf("corrupted journal file: failed to detect file version: %s", err)
		return
	}

	// read the file
	r, err := newDecoder(fd, fileVersion, startTime, endTime)
	if err != nil {
		errCh <- err
		return
	}
	for {
		if err := ctx.Err(); err != nil {
			return
		}

		// read the entry
		entry, bytesScanned, bytesProcessed, err := r.ReadEntry()
		if errors.Is(err, io.EOF) {
			return
		} else if err != nil {
			errCh <- fmt.Errorf("failed to read '%s': %w", fk.Filename(), err)
			return
		}

		// emit notice for bytes processed
		s.totalBytesScanned += bytesScanned
		s.totalBytesProcessed += bytesProcessed

		if s.totalBytesScanned >= s.nextScanNotice || s.totalBytesProcessed >= s.nextProcessedNotice {
			s.pushByteNotice()
			s.nextProcessedNotice = s.totalBytesProcessed + (chunkSize - (s.totalBytesProcessed % chunkSize))
			s.nextScanNotice = s.totalBytesScanned + (chunkSize - (s.totalBytesScanned % chunkSize))
		}

		// return the model
		entryCh <- *entry
	}
}

func (s *Scanner) pushByteNotice() {
	var str string
	if s.totalBytesScanned >= gb || s.totalBytesProcessed >= gb {
		str = fmt.Sprintf("%.2f gb scanned, %.2f gb processed", float64(s.totalBytesScanned)/gb, float64(s.totalBytesProcessed)/gb)
	} else {
		str = fmt.Sprintf("%.2f mb scanned, %.2f mb processed", float64(s.totalBytesScanned)/mb, float64(s.totalBytesProcessed)/mb)
	}
	s.noticeCh <- str
}

func detectJournalFileVersion(fd *os.File) (version int, err error) {
	var (
		arr [4]byte
		buf = arr[:]
	)

	_, err = io.ReadFull(fd, buf)
	if err != nil {
		return
	}

	if !bytes.Equal(buf[0:3], []byte("plj")) {
		return 0, errors.New("invalid or corrupted file")
	}

	// the fourth byte will be the version
	version = int(buf[3])

	return
}
