package logjournal

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"time"
)

var _ decoder = (*decoder2)(nil)

type decoder interface {
	ReadEntry() (entry *Entry, bytesScanned, bytesProcessed int64, err error)
}

// returns a new decoder using the specified version
func newDecoder(fd *os.File, version int, startTime, endTime time.Time) (decoder, error) {
	switch version {
	case 2:
		return &decoder2{
			fd: fd,

			startTime: startTime,
			endTime:   endTime,
		}, nil
	default:
		panic(fmt.Errorf("unknown file version: %d", version))
	}
}

// a reader for journal version 2 files
type decoder2 struct {
	fd *os.File

	startTime, endTime time.Time

	bytesScanned, bytesProcessed int64
}

func (r *decoder2) ReadEntry() (entry *Entry, bytesScanned, bytesProcessed int64, err error) {
	startBytesProcessed := r.bytesProcessed
	startBytesScanned := r.bytesScanned
	defer func() {
		bytesProcessed = r.bytesProcessed - startBytesProcessed
		bytesScanned = r.bytesScanned - startBytesScanned
	}()

	for {
		// read the header
		var (
			header     [16]byte
			ts         time.Time
			payloadLen int64
		)
		_, err = io.ReadFull(r.fd, header[:])
		if err != nil {
			return
		}

		payloadLen = int64(binary.LittleEndian.Uint64(header[0:8]))
		if payloadLen < 0 {
			panic(errors.New("negative payloadLen; this should never happen"))
		}
		ts = time.UnixMicro(int64(binary.LittleEndian.Uint64(header[8:16])))

		r.bytesProcessed += int64(len(header))
		r.bytesScanned += int64(len(header))

		// apply time-based filtering
		if isNotWithinTimeBounds(r.startTime, ts, r.endTime) {
			// skip this entry
			_, err = r.fd.Seek(payloadLen, io.SeekCurrent)
			if err != nil {
				return
			}
			continue
		}

		// read the entry
		payload := make([]byte, payloadLen)
		_, err = io.ReadFull(r.fd, payload)
		if err != nil {
			return
		}

		entry = new(Entry)
		err = json.Unmarshal(payload, entry)
		if err != nil {
			return
		}

		r.bytesProcessed += payloadLen

		// emit the entry
		return
	}
}
