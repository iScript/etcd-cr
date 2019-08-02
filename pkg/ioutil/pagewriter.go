package ioutil

import (
	"io"
)

var defaultBufferBytes = 128 * 1024

// PageWriter implements the io.Writer interface so that writes will
// either be in page chunks or from flushing.
type PageWriter struct {
	w io.Writer
	// pageOffset tracks the page offset of the base of the buffer
	pageOffset int
	// pageBytes is the number of bytes per page
	pageBytes int
	// bufferedBytes counts the number of bytes pending for write in the buffer
	bufferedBytes int
	// buf holds the write buffer
	buf []byte
	// bufWatermarkBytes is the number of bytes the buffer can hold before it needs
	// to be flushed. It is less than len(buf) so there is space for slack writes
	// to bring the writer to page alignment.
	bufWatermarkBytes int
}

// NewPageWriter creates a new PageWriter. pageBytes is the number of bytes
// to write per page. pageOffset is the starting offset of io.Writer.
func NewPageWriter(w io.Writer, pageBytes, pageOffset int) *PageWriter {
	return &PageWriter{
		w:                 w,
		pageOffset:        pageOffset,
		pageBytes:         pageBytes,
		buf:               make([]byte, defaultBufferBytes+pageBytes),
		bufWatermarkBytes: defaultBufferBytes,
	}
}
