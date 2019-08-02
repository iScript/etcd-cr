package wal

import (
	"hash"
	"io"
	"os"
	"sync"

	"github.com/iScript/etcd-cr/pkg/ioutil"
)

const walPageBytes = 8 * minSectorSize

type encoder struct {
	mu sync.Mutex
	bw *ioutil.PageWriter

	crc       hash.Hash32
	buf       []byte
	uint64buf []byte
}

func newEncoder(w io.Writer, prevCrc uint32, pageOffset int) *encoder {
	return &encoder{
		bw: ioutil.NewPageWriter(w, walPageBytes, pageOffset),
		//crc: crc.New(prevCrc, crcTable),
		// 1MB buffer
		buf:       make([]byte, 1024*1024),
		uint64buf: make([]byte, 8),
	}
}

// newFileEncoder creates a new encoder with current file offset for the page writer.
func newFileEncoder(f *os.File, prevCrc uint32) (*encoder, error) {
	offset, err := f.Seek(0, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	return newEncoder(f, prevCrc, int(offset)), nil
}

// func (e *encoder) encode(rec *walpb.Record) error {
// 	e.mu.Lock()
// 	defer e.mu.Unlock()

// 	e.crc.Write(rec.Data)
// 	rec.Crc = e.crc.Sum32()
// 	var (
// 		data []byte
// 		err  error
// 		n    int
// 	)

// 	if rec.Size() > len(e.buf) {
// 		data, err = rec.Marshal()
// 		if err != nil {
// 			return err
// 		}
// 	} else {
// 		n, err = rec.MarshalTo(e.buf)
// 		if err != nil {
// 			return err
// 		}
// 		data = e.buf[:n]
// 	}

// 	lenField, padBytes := encodeFrameSize(len(data))
// 	if err = writeUint64(e.bw, lenField, e.uint64buf); err != nil {
// 		return err
// 	}

// 	if padBytes != 0 {
// 		data = append(data, make([]byte, padBytes)...)
// 	}
// 	_, err = e.bw.Write(data)
// 	return err
// }
