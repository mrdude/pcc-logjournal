package logjournal

import (
	"context"
	"errors"
	"os"
	"slices"
	"time"

	"git.home.prztl.com/git/w/prztl-common-go/pcommon"
	"go.uber.org/zap"
)

// returns a list of deleted filekeys
func executeGC(ctx context.Context, dir *dir) ([]PartitionKey, error) {
	logger := pcommon.GetLogger(ctx)

	var deleted []PartitionKey

	// calculate the earliest filekey we will allow
	const day = 24 * time.Hour
	deleteBefore := PartitionKeyFromTimestamp(time.Now().UTC().Add(-7 * day)) // TODO bump this back up to 60 days once the logserver has more disk
	deleteBefore.segment = -1

	// list all filekeys
	filekeys, err := dir.ListFileKeys()
	if err != nil {
		return nil, err
	}

	// delete any filekeys that are older than 60 days
	i, found := slices.BinarySearchFunc(filekeys, deleteBefore, ComparePartitionKeys)
	if found {
		// deleteBefore has a segment=-1, and those never get returned from dir.ListFileKeys()
		panic(errors.New("this should never happen"))
	}
	i--

	if i > 0 {
		for _, fk := range filekeys[:i] {
			logger.Info("Deleting journal file for being too old",
				zap.String("filekey", fk.String()))

			err = dir.Remove(fk)
			if err != nil {
				return nil, err
			}

			deleted = append(deleted, fk)
		}
	}

	// update the filekeys list
	filekeys, err = dir.ListFileKeys()
	if err != nil {
		return nil, err
	}

	// if the total journal size is above the high watermark, start deleting files
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024

		highWatermark = 15 * gb // TODO set back to 50GB once we have more disk space
		lowWatermark  = 10 * gb // TODO set back to 40GB once we have more disk space
	)
	var (
		fileSizes                        = make(map[PartitionKey]int64) // map of fileKey -> size of that file, for all fileKeys we have already scanned
		totalJournalSize           int64 = 0
		deleteBeforeIndexExclusive       = 0
	)
	for _, fk := range filekeys {
		// get the file size
		var info os.FileInfo
		info, err = dir.Stat(fk)
		if err != nil {
			// TODO should this stop us from deleting files?
			return nil, err
		}

		fileSizes[fk] = info.Size()
		totalJournalSize += info.Size()

		// if we have exceeded the high watermark, delete files until we hit the low watermark
		if totalJournalSize > highWatermark {
			for totalJournalSize > lowWatermark && deleteBeforeIndexExclusive < len(filekeys) {
				// mark the earliest file we have left for deletion
				sz := fileSizes[filekeys[deleteBeforeIndexExclusive]]
				totalJournalSize -= sz
				deleteBeforeIndexExclusive++
			}
		}
	}

	// delete the files we have marked
	for _, fk := range filekeys[:deleteBeforeIndexExclusive] {
		logger.Info("Deleting journal file for taking too much space",
			zap.String("filekey", fk.String()))

		err = dir.Remove(fk)
		if err != nil {
			return nil, err
		}

		deleted = append(deleted, fk)
	}

	return deleted, nil
}
