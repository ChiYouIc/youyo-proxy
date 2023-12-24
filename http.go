package youyoproxy

import (
	"io"
	"net/http"
)

func (proxy *HttpProxy) handleHttp(rw http.ResponseWriter, r *http.Request) {

	// todo websocket request

	// remove proxy handler
	removeProxyHeaders(r)

	proxy.Info("http request: %50s %10s %s", r.Host, r.Method, r.RequestURI)

	resp, err := proxy.Tr.RoundTrip(r)
	if resp == nil {
		var errorString string
		if err != nil {
			errorString = "error read response " + r.URL.Host + " : " + err.Error()
			http.Error(rw, err.Error(), 500)
		} else {
			errorString = "error read response " + r.URL.Host
			proxy.Error(errorString)
			http.Error(rw, errorString, 500)
		}
		return
	}

	resp = proxy.filterResponse(resp)

	copyHeaders(rw.Header(), resp.Header)
	rw.WriteHeader(resp.StatusCode)
	var copyWriter io.Writer = rw
	if rw.Header().Get("content-type") == "text/event-stream" {
		// server-side events, flush the buffered data to the client.
		copyWriter = &flushWriter{w: rw}
	}

	nr, err := io.Copy(copyWriter, resp.Body)
	if err := resp.Body.Close(); err != nil {
		proxy.Error("Can't close response body %v", err)
	}
	proxy.Debugger("Copied %v bytes to client error=%v", nr, err)
}

type flushWriter struct {
	w io.Writer
}

func (fw flushWriter) Write(p []byte) (int, error) {
	n, err := fw.w.Write(p)
	if f, ok := fw.w.(http.Flusher); ok {
		// only flush if the Writer implements the Flusher interface.
		f.Flush()
	}

	return n, err
}
