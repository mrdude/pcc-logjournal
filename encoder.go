package logjournal

import (
	"encoding/binary"
	"errors"
	"os"
)

var _ encoder = (*encoder2)(nil)

type encoder interface {
	WriteEntries(entries []Entry) error
	BytesWritten() int64
	Sync() error
	Close() error
}

// returns a new instance of the default encoder
func newEncoder(fd *os.File) (encoder, error) {
	return newEncoderVersion(fd, 2)
}

func newEncoderVersion(fd *os.File, fileVersion int) (encoder, error) {
	// write the file header
	buf := []byte("plj0")
	buf[3] = byte(fileVersion)
	_, err := fd.Write(buf)
	if err != nil {
		return nil, err
	}

	// return the encoder
	switch fileVersion {
	case 2:
		return &encoder2{
			fd: fd,
		}, nil
	default:
		panic(errors.New("this should never happen"))
	}
}

// a journal file writer that encodes version 2 files
//
// a version 2 file is an uncompressed stream of
// Entry's. Each Entry is prefixed with a header containing
// the payload length and timestamp.
//
// not using compression allows decoder2 to use
// os.File.Seek to skip unneeded entries
type encoder2 struct {
	fd           *os.File
	bytesWritten int64
}

func (w *encoder2) WriteEntries(entries []Entry) error {
	// marshal each entry and append it to the buffer
	var buf []byte

	for _, e := range entries {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(len(e.Data)))
		buf = binary.LittleEndian.AppendUint64(buf, uint64(e.Ts.UnixMicro()))
		buf = append(buf, e.Data...)
	}

	// write the buffer
	n, err := w.fd.Write(buf)
	if err == nil {
		w.bytesWritten += int64(n)
	}
	return err
}

func (w *encoder2) BytesWritten() int64 {
	return w.bytesWritten
}

func (w *encoder2) Sync() error {
	if err := w.fd.Sync(); err != nil {
		return err
	}

	return nil
}

func (w *encoder2) Close() error {
	return w.fd.Close()
}
