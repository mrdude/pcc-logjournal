package logjournal

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// dir represents a directory containing Journal files
// all dir receiver methods are thread-safe
type dir struct {
	// protects cachedKeys and cachedKeysUpdate
	mu sync.Mutex

	d string

	// a cached list of PartitionKey's in the directory
	cachedKeys []PartitionKey

	// when is the last time we updated cachedKeys?
	cachedKeysUpdate time.Time
}

func (jd *dir) Dir() string {
	return string(jd.d)
}

// ListFileKeys returns a sorted list of fileKey's in the journal directory
func (jd *dir) ListFileKeys() (keys []PartitionKey, err error) {
	jd.mu.Lock()
	defer jd.mu.Unlock()

	// check the cache
	const cacheTime = 5 * time.Minute // TODO: this is completely arbitrary -- is there a better value?
	if time.Since(jd.cachedKeysUpdate) < cacheTime {
		keys = append(keys, jd.cachedKeys...)
		return
	}

	// update the cache
	err = fs.WalkDir(os.DirFS(jd.Dir()), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		path = filepath.Join(jd.Dir(), path)
		fk, err := PartitionKeyFromFilename(path)
		if err != nil {
			return err
		}

		keys = append(keys, fk)
		return nil
	})
	if err != nil {
		return
	}

	// sort the file keys in ascending order
	slices.SortFunc(keys, ComparePartitionKeys)

	// update the cache
	jd.cachedKeys = make([]PartitionKey, 0, len(keys))
	jd.cachedKeys = append(jd.cachedKeys, keys...)
	jd.cachedKeysUpdate = time.Now()

	return
}

// NextKeyForDate returns the next available fileKey for the given date
// TODO test this
func (jd *dir) NextKeyForDate(current PartitionKey) (fk PartitionKey, err error) {
	// gather all partition keys
	var keys []PartitionKey
	keys, err = jd.ListFileKeys()
	if err != nil {
		err = fmt.Errorf("error while listing filekeys: %w", err)
		return
	}

	fk = nextKeyForDate(current, keys)
	return
}

// TODO test this more
func nextKeyForDate(current PartitionKey, keys []PartitionKey) (fk PartitionKey) {
	if len(keys) == 0 {
		return current
	}

	// create a fileKey
	fk = current
	fk.segment = -1 // this will make fk sort to the end of the .Date()

	// find the last fileKey for this Date
	i, found := slices.BinarySearchFunc(keys, fk, ComparePartitionKeys)
	//if i == 0 {
	//	fk.segment = 0
	//	return
	//}
	if found {
		i--
	}

	// set segment
	if keys[i].Date() == fk.Date() {
		// there already exists a fileKey with this date
		fk.segment = keys[i].segment + 1
	} else {
		// there isn't already a fileKey with this date
		fk.segment = 0
	}

	return
}

// OpenFile opens a file for writing
func (jd *dir) OpenFile(fk PartitionKey) (fd *os.File, err error) {
	filename := filepath.Join(jd.Dir(), fk.Filename())

	fd, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err == nil {
		jd.mu.Lock()
		i, found := slices.BinarySearchFunc(jd.cachedKeys, fk, ComparePartitionKeys)
		if !found {
			jd.cachedKeys = slices.Insert(jd.cachedKeys, i, fk)
		}
		jd.mu.Unlock()
	}
	return
}

// Stat runs os.Stat on the given file
func (jd *dir) Stat(fk PartitionKey) (os.FileInfo, error) {
	filename := filepath.Join(jd.Dir(), fk.Filename())
	return os.Stat(filename)
}

// Remove removes the given file
func (jd *dir) Remove(fk PartitionKey) error {
	filename := filepath.Join(jd.Dir(), fk.Filename())
	err := os.Remove(filename)
	if err == nil {
		jd.mu.Lock()
		i, found := slices.BinarySearchFunc(jd.cachedKeys, fk, ComparePartitionKeys)
		if found {
			jd.cachedKeys = slices.Delete(jd.cachedKeys, i, i+1)
		}
		jd.mu.Unlock()
	}
	return err
}
