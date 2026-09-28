// Package httpgzip compresses the responses a client accepts compressed.
//
// The interface bundle weighs about four times more raw than gzipped, and the
// task list is JSON that repeats the same keys for every task. Compressing them
// is what makes a first load, or the first load after a release, cheap.
//
// The decision is taken per response, on the first bytes the handler writes,
// because only then are its headers known. A response is left alone when it is
// already encoded (the metrics endpoint encodes itself), when it is a stream
// whose events must reach the client as they happen (text/event-stream), when
// its type gains nothing from gzip (images, archives), or when it is too small
// for the gzip framing to pay for itself.
package httpgzip

import (
	"bufio"
	"compress/gzip"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// minSize is the body size below which a response is sent as is: gzip adds
// about twenty bytes of framing, and a short body barely shrinks.
const minSize = 1024

var writers = sync.Pool{New: func() any { return gzip.NewWriter(io.Discard) }}

// Handler compresses what next answers, for the clients that accept gzip.
func Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// A cache in front must key the body on the header that chose it, even
		// when this particular answer ends up uncompressed.
		w.Header().Add("Vary", "Accept-Encoding")
		// A byte range addresses the plain file: compressing the slice would
		// hand the client bytes that match no offset it asked for. A WebSocket
		// upgrade leaves HTTP altogether, and HEAD has no body to compress.
		if !acceptsGzip(r.Header.Get("Accept-Encoding")) ||
			r.Header.Get("Range") != "" ||
			r.Header.Get("Upgrade") != "" ||
			r.Method == http.MethodHead {
			next.ServeHTTP(w, r)
			return
		}
		cw := &responseWriter{ResponseWriter: w}
		defer cw.close()
		next.ServeHTTP(cw, r)
	})
}

// acceptsGzip reads an Accept-Encoding header. gzip, or the "*" wildcard, is
// accepted unless its quality is zero; an explicit gzip entry wins over "*".
func acceptsGzip(header string) bool {
	gzipQuality, wildcardQuality := -1.0, -1.0
	for _, part := range strings.Split(header, ",") {
		coding, params, _ := strings.Cut(strings.TrimSpace(part), ";")
		quality := 1.0
		if name, value, ok := strings.Cut(strings.TrimSpace(params), "="); ok && strings.TrimSpace(name) == "q" {
			parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				continue
			}
			quality = parsed
		}
		switch strings.ToLower(strings.TrimSpace(coding)) {
		case "gzip", "x-gzip":
			gzipQuality = quality
		case "*":
			wildcardQuality = quality
		}
	}
	if gzipQuality >= 0 {
		return gzipQuality > 0
	}
	return wildcardQuality > 0
}

// compressible reports whether a response with these headers is worth
// compressing. It is asked once the handler has set them.
func compressible(h http.Header) bool {
	if h.Get("Content-Encoding") != "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(h.Get("Content-Type"))
	if err != nil {
		return false
	}
	switch {
	case mediaType == "text/event-stream":
		return false
	case strings.HasPrefix(mediaType, "text/"),
		strings.HasSuffix(mediaType, "+json"),
		strings.HasSuffix(mediaType, "+xml"):
		return true
	}
	switch mediaType {
	case "application/json", "application/javascript", "application/x-javascript",
		"application/xml", "application/wasm", "image/svg+xml":
		return true
	}
	return false
}

// responseWriter holds the first bytes of a response until it knows whether to
// compress it, then either streams them through gzip or hands them over as is.
type responseWriter struct {
	http.ResponseWriter

	status   int  // the status the handler asked for, 0 until it does
	decided  bool // the headers have gone out, and with them the choice
	hijacked bool
	buf      []byte
	gz       *gzip.Writer
}

func (w *responseWriter) WriteHeader(code int) {
	if w.decided {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	// An informational status is not the answer: it goes out at once and the
	// handler still owes the real one.
	if code >= 100 && code < 200 && code != http.StatusSwitchingProtocols {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.status == 0 {
		w.status = code
	}
	if !bodyAllowed(code) {
		w.commit(false)
	}
}

func (w *responseWriter) Write(p []byte) (int, error) {
	if w.decided {
		if w.gz != nil {
			return w.gz.Write(p)
		}
		return w.ResponseWriter.Write(p)
	}
	w.buf = append(w.buf, p...)
	if len(w.buf) >= minSize {
		if err := w.decide(); err != nil {
			return 0, err
		}
	}
	return len(p), nil
}

// Flush sends what is buffered now: a handler that flushes wants its bytes on
// the wire, whatever their size.
func (w *responseWriter) Flush() {
	if !w.decided {
		if err := w.decide(); err != nil {
			return
		}
	}
	if w.gz != nil {
		_ = w.gz.Flush()
	}
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("httpgzip: the underlying ResponseWriter does not support hijacking")
	}
	conn, rw, err := hijacker.Hijack()
	if err == nil {
		w.hijacked = true
	}
	return conn, rw, err
}

func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

// decide settles the encoding from the headers and the buffered bytes, sends
// the headers, and writes out the buffer.
func (w *responseWriter) decide() error {
	h := w.Header()
	// Without a type, net/http would guess it from the first bytes it writes,
	// which would be gzip's. Guess it here, from the plain bytes.
	if h.Get("Content-Type") == "" && len(w.buf) > 0 {
		h.Set("Content-Type", http.DetectContentType(w.buf))
	}
	w.commit(compressible(h) && !declaredSmall(h))
	buf := w.buf
	w.buf = nil
	if len(buf) == 0 {
		return nil
	}
	var err error
	if w.gz != nil {
		_, err = w.gz.Write(buf)
	} else {
		_, err = w.ResponseWriter.Write(buf)
	}
	return err
}

// commit sends the headers, rewritten for gzip when compress is set.
func (w *responseWriter) commit(compress bool) {
	w.decided = true
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if compress {
		h := w.Header()
		h.Set("Content-Encoding", "gzip")
		// Both describe the plain body: its length and its byte offsets.
		h.Del("Content-Length")
		h.Del("Accept-Ranges")
		// A strong validator promises identical bytes, which the encoded body
		// no longer is.
		if etag := h.Get("ETag"); etag != "" && !strings.HasPrefix(etag, "W/") {
			h.Set("ETag", "W/"+etag)
		}
		gz := writers.Get().(*gzip.Writer)
		gz.Reset(w.ResponseWriter)
		w.gz = gz
	}
	w.ResponseWriter.WriteHeader(w.status)
}

// close finishes the response once the handler has returned: a body that never
// reached minSize goes out as is, a compressed one gets its gzip trailer.
func (w *responseWriter) close() {
	if w.hijacked {
		return
	}
	if !w.decided {
		if len(w.buf) < minSize {
			w.commit(false)
			if len(w.buf) > 0 {
				_, _ = w.ResponseWriter.Write(w.buf)
			}
			w.buf = nil
			return
		}
		_ = w.decide()
	}
	if w.gz != nil {
		_ = w.gz.Close()
		w.gz.Reset(io.Discard)
		writers.Put(w.gz)
		w.gz = nil
	}
}

// declaredSmall reports a Content-Length the handler set below minSize.
func declaredSmall(h http.Header) bool {
	length, err := strconv.ParseInt(h.Get("Content-Length"), 10, 64)
	return err == nil && length < minSize
}

func bodyAllowed(status int) bool {
	return status != http.StatusNoContent &&
		status != http.StatusNotModified &&
		status != http.StatusSwitchingProtocols
}
