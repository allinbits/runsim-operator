package io

import "io"

// TeeReader has a similar behaviour as io.TeeReader with the exception
// that it stops writing to writer when it is closed
func TeeReader(r io.Reader, w io.Writer) io.Reader {
	return &teeReader{r: r, w: w}
}

type teeReader struct {
	r           io.Reader
	w           io.Writer
	writeClosed bool
}

func (t *teeReader) Read(p []byte) (n int, err error) {
	n, err = t.r.Read(p)
	if !t.writeClosed && n > 0 {
		if n, err := t.w.Write(p[:n]); err != nil {
			if err != io.ErrClosedPipe {
				return n, err

			}
			t.writeClosed = true
		}
	}
	return
}
