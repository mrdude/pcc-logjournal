package logjournal

import (
	"io"
)

// indexWriter handles writing model.LogEntry's to disk,
// where all model.LogEntry's are for the same PartitionKey.Date()
type indexWriter struct {
	journalDir *dir

	baseKey    PartitionKey // the PartitionKey, with segment=0
	currentKey PartitionKey // the current PartitionKey that we are writing to

	fd encoder // the current file we are writing to
}

// newIndexWriter opens an indexWriter for appending
func newIndexWriter(d *dir, fk PartitionKey) (iw *indexWriter, err error) {
	fk.segment = 0

	iw = &indexWriter{
		journalDir: d,
		baseKey:    fk,
	}

	iw.baseKey.segment = 0
	iw.currentKey = iw.baseKey
	iw.currentKey, err = iw.journalDir.NextKeyForDate(iw.currentKey) // seek the currentKey to the end of this date
	if err != nil {
		return
	}

	// open the first file
	iw.currentKey.segment -= 1 // openFile() will increment segment before opening the file
	err = iw.openFile()
	return
}

func (j *indexWriter) openFile() error {
	// close the current file, if there is one
	j.Close()

	// open a new file
	j.currentKey.segment++
	fd, err := j.journalDir.OpenFile(j.currentKey)
	if err != nil {
		return err
	}

	// seek to the end of the file
	// TODO this shouldn't be necessary, because the file should always be empty
	_, err = fd.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}

	// set the file descriptor
	j.fd, err = newEncoder(fd)
	if err != nil {
		return err
	}
	return nil
}

func (j *indexWriter) Close() {
	if j.fd != nil {
		j.fd.Close()
		j.fd = nil
	}
}

func (j *indexWriter) Append(entries []Entry) error {
	// open the journal file, if needed
	if j.fd == nil {
		if err := j.openFile(); err != nil {
			return err
		}
	}

	// write the buffer to the current journal file
	// TODO return error if any entries are in the wrong fileKey?
	err := j.fd.WriteEntries(entries)

	// if the file write failed, close the file
	if err != nil {
		j.Close()
		return err
	}

	// flush
	if err := j.fd.Sync(); err != nil {
		j.Close()
		return err
	}

	// rotate the file if it is too big
	const maxJournalFileSize = 1024 * 1024 * 512 // 512MB
	if j.fd.BytesWritten() >= maxJournalFileSize {
		j.Close()
	}

	return nil
}
