package transport

import (
	"bytes"
	"context"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"sort"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type DeferedMultipartWriter struct {
	pipe <-chan graphql.ResponseHandler
	exec graphql.GraphExecutor

	total     int
	processed int
	w         http.ResponseWriter
	mw        *multipart.Writer
}

func (d *DeferedMultipartWriter) First(r *graphql.Response) {
	if err := d.write(r); err != nil {
		panic(err)
	}
}

func (d *DeferedMultipartWriter) Dispatch(ctx context.Context, w http.ResponseWriter) {
	for resp := range d.pipe {
		if flusher, ok := d.w.(http.Flusher); ok {
			flusher.Flush()
		}

		if err := d.write(resp(ctx)); err != nil {
			panic(err)
		}
	}

	if err := d.mw.Close(); err != nil {
		gqlErr := gqlerror.Errorf("closing multipart response: %+v", err)
		resp := d.exec.DispatchError(ctx, gqlerror.List{gqlErr})
		writeJson(w, resp)
		return
	}
}

func (d *DeferedMultipartWriter) write(r *graphql.Response) error {
	r.HasNext = d.processed != d.total
	d.processed++

	buf := bytes.NewBuffer(nil)
	writeJson(buf, r)

	headers := textproto.MIMEHeader{}
	headers.Add("Content-Type", "application/json")
	headers.Add("Content-Length", strconv.Itoa(buf.Len()))

	if d.processed == 1 {
		fmt.Fprintf(d.w, "--%s\r\n", d.mw.Boundary())
	}

	keys := make([]string, 0, len(headers))
	for k := range headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, v := range headers[k] {
			fmt.Fprintf(d.w, "%s: %s\r\n", k, v)
		}
	}
	fmt.Fprintf(d.w, "\r\n")

	d.w.Write(buf.Bytes())

	if r.HasNext {
		fmt.Fprintf(d.w, "\r\n--%s\r\n", d.mw.Boundary())
	}

	if flusher, ok := d.w.(http.Flusher); ok {
		flusher.Flush()
	}

	return nil
}

func NewDeferedMultipartWriter(w http.ResponseWriter, t int, exec graphql.GraphExecutor, c <-chan graphql.ResponseHandler) *DeferedMultipartWriter {
	mw := multipart.NewWriter(w)
	w.Header().Set("Content-Type", "multipart/mixed; boundary="+mw.Boundary())

	return &DeferedMultipartWriter{w: w, total: t, pipe: c, exec: exec, mw: mw}
}
