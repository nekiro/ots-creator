package collab

import (
	"bytes"
	"compress/flate"
	"errors"
	"io"
	"runtime"
	"slices"
	"sync"

	"github.com/nekiro/ots-creator/internal/formats/binio"
)

// A joining guest gets the map as the OTMAP text files (see
// otmap.EncodeOTMAP), in compressed parts of about partSize bytes each:
// a part holds its index, the number of parts and the compressed records
// (file name and content; the world file has no name).

const partSize = 4 << 20

// Snapshot is the map a guest received: the world file and the sector
// files by name.
type Snapshot struct {
	World   []byte
	Sectors map[string][]byte
	// Seq is the last edit the snapshot contains.
	Seq uint64
}

type snapshotRecord struct {
	name string
	data []byte
}

// encodeSnapshot splits the map into compressed parts, compressing on all
// cores.
func encodeSnapshot(world []byte, sectors map[string][]byte) ([][]byte, error) {
	names := make([]string, 0, len(sectors))
	for name := range sectors {
		names = append(names, name)
	}
	slices.Sort(names)
	var groups [][]snapshotRecord
	current := []snapshotRecord{{"", world}}
	size := len(world)
	for _, name := range names {
		if size >= partSize {
			groups = append(groups, current)
			current, size = nil, 0
		}
		current = append(current, snapshotRecord{name, sectors[name]})
		size += len(sectors[name])
	}
	groups = append(groups, current)
	parts := make([][]byte, len(groups))
	errs := make([]error, len(groups))
	var wg sync.WaitGroup
	limit := make(chan struct{}, max(runtime.GOMAXPROCS(0)-1, 1))
	for i, group := range groups {
		wg.Add(1)
		limit <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-limit }()
			parts[i], errs[i] = encodePart(i, len(groups), group)
		}()
	}
	wg.Wait()
	return parts, errors.Join(errs...)
}

func encodePart(index, count int, records []snapshotRecord) ([]byte, error) {
	raw := binio.NewWriter(partSize + 4096)
	raw.U32(uint32(len(records)))
	for _, record := range records {
		putString(raw, record.name)
		putBytes(raw, record.data)
	}
	var out bytes.Buffer
	header := binio.NewWriter(8)
	header.U32(uint32(index))
	header.U32(uint32(count))
	out.Write(header.Bytes())
	compressor, err := flate.NewWriter(&out, 3)
	if err != nil {
		return nil, err
	}
	if _, err := compressor.Write(raw.Bytes()); err != nil {
		return nil, err
	}
	if err := compressor.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// snapshotReader collects the parts of a snapshot.
type snapshotReader struct {
	snapshot Snapshot
	parts    int
	count    int
	size     int
}

var errSnapshot = errors.New("the map from the host is damaged")

// add reads one part; it returns the parts read and the number of parts.
func (reader *snapshotReader) add(payload []byte) (done, total int, err error) {
	if len(payload) < 8 {
		return 0, 0, errSnapshot
	}
	header := binio.NewReader(payload[:8])
	index, count := int(header.U32()), int(header.U32())
	if count == 0 || index != reader.parts || (reader.count != 0 && count != reader.count) {
		return 0, 0, errSnapshot
	}
	reader.count = count
	// The uncompressed size is bounded: a small part must not unpack into
	// gigabytes.
	limit := int64(maxSnapshotData - reader.size)
	data, err := io.ReadAll(io.LimitReader(flate.NewReader(bytes.NewReader(payload[8:])), limit+1))
	if err != nil {
		return 0, 0, errSnapshot
	}
	if int64(len(data)) > limit {
		return 0, 0, errors.New("the map from the host is too big")
	}
	reader.size += len(data)
	r := binio.NewReader(data)
	records := getCount(r, 8)
	if reader.snapshot.Sectors == nil {
		reader.snapshot.Sectors = map[string][]byte{}
	}
	for range records {
		name := getString(r)
		content := getBytes(r)
		if r.Err() != nil {
			break
		}
		if name == "" {
			reader.snapshot.World = content
		} else {
			reader.snapshot.Sectors[name] = content
		}
	}
	if r.Err() != nil {
		return 0, 0, errSnapshot
	}
	reader.parts++
	return reader.parts, reader.count, nil
}

func (reader *snapshotReader) complete() bool {
	return reader.count > 0 && reader.parts == reader.count && reader.snapshot.World != nil
}
