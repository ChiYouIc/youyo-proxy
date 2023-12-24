package youyoproxy

import (
	"bufio"
	"crypto/tls"
	"io"
	"net/http"
)

type HttpProxy struct {
	Tr            *http.Transport
	TLSConfig     func(host string) (*tls.Config, error)
	RespHandlers  []RespHandler
	httpsHandlers []HttpsHandler
	IsDebugger    bool
}

func (proxy *HttpProxy) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodConnect {
		proxy.handleHttps(rw, r)
	} else {
		proxy.handleHttp(rw, r)
	}
}

func (proxy *HttpProxy) filterResponse(respOrig *http.Response) (resp *http.Response) {
	resp = respOrig
	for _, h := range proxy.RespHandlers {
		resp = h.Handle(resp)
	}
	return
}

func NewHttpProxy() *HttpProxy {

	proxy := &HttpProxy{
		Tr: &http.Transport{
			TLSClientConfig: tlsClientSkipVerify,
			Proxy:           http.ProxyFromEnvironment,
		},
		RespHandlers: []RespHandler{},
	}

	proxy.TLSConfig = TLSConfigFromCA(&GoproxyCa, proxy)

	return proxy
}

// isEof 是否文件结尾
func isEof(r *bufio.Reader) bool {
	_, err := r.Peek(1)
	if err == io.EOF {
		return true
	}
	return false
}

func removeProxyHeaders(r *http.Request) {
	r.RequestURI = "" // this must be reset when serving a request with the client
	// If no Accept-Encoding header exists, Transport will add the headers it can accept
	// and would wrap the response body with the relevant reader.
	r.Header.Del("Accept-Encoding")
	// curl can add that, see
	// https://jdebp.eu./FGA/web-proxy-connection-header.html
	r.Header.Del("Proxy-Connection")
	r.Header.Del("Proxy-Authenticate")
	r.Header.Del("Proxy-Authorization")
	// Connection, Authenticate and Authorization are single hop Header:
	// http://www.w3.org/Protocols/rfc2616/rfc2616.txt
	// 14.10 Connection
	//   The Connection general-header field allows the sender to specify
	//   options that are desired for that particular connection and MUST NOT
	//   be communicated by proxies over further connections.

	// When server reads http request it sets req.Close to true if
	// "Connection" header contains "close".
	// https://github.com/golang/go/blob/master/src/net/http/request.go#L1080
	// Later, transfer.go adds "Connection: close" back when req.Close is true
	// https://github.com/golang/go/blob/master/src/net/http/transfer.go#L275
	// That's why tests that checks "Connection: close" removal fail
	if r.Header.Get("Connection") == "close" {
		r.Close = false
	}
	r.Header.Del("Connection")
}

func copyHeaders(dst, src http.Header) {
	for k := range dst {
		dst.Del(k)
	}

	for k, vs := range src {
		for _, v := range vs {
			dst.Add(k, v)
		}
	}
}
