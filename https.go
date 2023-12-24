package youyoproxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var httpsRegexp = regexp.MustCompile(`^https:\/\/`)

type ConnectActionLiteral int

// ConnectAction enables the caller to override the standard connect flow.
// When Action is ConnectHijack, it is up to the implementer to send the
// HTTP 200, or any other valid http response back to the client from within the
// Hijack func
type ConnectAction struct {
	Action    ConnectActionLiteral
	Hijack    func(req *http.Request, client net.Conn)
	TLSConfig func(host string) (*tls.Config, error)
}

func (proxy *HttpProxy) handleHttps(rw http.ResponseWriter, r *http.Request) {
	hij, ok := rw.(http.Hijacker)
	if !ok {
		panic("httpserver does not support hijacking")
	}

	proxyClient, _, e := hij.Hijack()
	if e != nil {
		panic("Cannot hijack connection " + e.Error())
	}

	// Connect Mitm 中间人
	_, err := proxyClient.Write([]byte("HTTP/1.0 200 OK\r\n\r\n"))
	if err != nil {
		httpError(proxyClient, proxy, err)
		return
	}

	tlsConfig, err := proxy.TLSConfig(r.URL.Host)
	if err != nil {
		httpError(proxyClient, proxy, err)
		return
	}
	go func() {
		//TODO: cache connections to the remote website

		// 创建一个新的 TLS 服务器端连接
		rawClientTls := tls.Server(proxyClient, tlsConfig)

		// Handshake 运行客户端或服务器 TLS 握手协议（如果尚未运行）。
		if err := rawClientTls.Handshake(); err != nil {
			proxy.Warn("Cannot handshake client %v %v", r.Host, err)
			return
		}
		defer rawClientTls.Close()

		// 获取连接的 Reader
		clientTlsReader := bufio.NewReader(rawClientTls)

		for !isEof(clientTlsReader) {
			// 从 TLS 服务连接中读取请求
			req, err := http.ReadRequest(clientTlsReader)
			if err != nil && err != io.EOF {
				return
			}

			if err != nil {
				proxy.Warn("Cannot read TLS request from mitm'd client %v %v", r.Host, err)
				return
			}

			// 由于我们正在转换请求，因此还需要保留原始连接 IP
			req.RemoteAddr = r.RemoteAddr
			proxy.Debugger("req %v", r.Host)

			if !httpsRegexp.MatchString(req.URL.String()) {
				req.URL, err = url.Parse("https://" + r.Host + req.URL.String())
			}

			proxy.Info("https request: %-25s %-10s %s", req.Host, req.Method, req.RequestURI)

			//todo websocket

			// 清除代理请求头
			removeProxyHeaders(req)

			// 执行请求
			resp, err := func() (*http.Response, error) {
				// explicitly discard request body to avoid data races in certain RoundTripper implementations
				// see https://github.com/golang/go/issues/61596#issuecomment-1652345131
				defer req.Body.Close()
				return proxy.Tr.RoundTrip(req)
			}()

			if err != nil {
				proxy.Warn("Cannot read TLS response from mitm'd server %v", err)
				return
			}

			resp = proxy.filterResponse(resp)

			// 响应信息
			proxy.Debugger("resp %v", resp.Status)
			defer resp.Body.Close()

			text := resp.Status
			statusCode := strconv.Itoa(resp.StatusCode) + " "
			if strings.HasPrefix(text, statusCode) {
				text = text[len(statusCode):]
			}
			// 始终使用 1.1 来支持分块编码
			if _, err := io.WriteString(rawClientTls, "HTTP/1.1"+" "+statusCode+text+"\r\n"); err != nil {
				proxy.Warn("Cannot write TLS response HTTP status from mitm'd client: %v", err)
				return
			}

			if resp.Request.Method == "HEAD" {
				// don't change Content-Length for HEAD request
			} else {
				// Since we don't know the length of resp, return chunked encoded response
				// TODO: use a more reasonable scheme
				resp.Header.Del("Content-Length")
				resp.Header.Set("Transfer-Encoding", "chunked")
			}

			// 强制连接关闭，否则 chrome 将使 CONNECT 隧道永远保持打开状态
			resp.Header.Set("Connection", "close")
			if err := resp.Header.Write(rawClientTls); err != nil {
				proxy.Warn("Cannot write TLS response header from mitm'd client: %v", err)
				return
			}

			if _, err = io.WriteString(rawClientTls, "\r\n"); err != nil {
				proxy.Warn("Cannot write TLS response header end from mitm'd client: %v", err)
				return
			}

			if resp.Request.Method == "HEAD" {
				// Don't write out a response body for HEAD request
			} else {
				chunked := newChunkedWriter(rawClientTls)
				if _, err := io.Copy(chunked, resp.Body); err != nil {
					proxy.Warn("Cannot write TLS response body from mitm'd client: %v", err)
					return
				}
				if err := chunked.Close(); err != nil {
					proxy.Warn("Cannot write TLS chunked EOF from mitm'd client: %v", err)
					return
				}
				if _, err = io.WriteString(rawClientTls, "\r\n"); err != nil {
					proxy.Warn("Cannot write TLS response chunked trailer from mitm'd client: %v", err)
					return
				}
			}

		}
		proxy.Debugger("Exiting on EOF")
	}()
}

func stripPort(s string) string {
	var ix int
	if strings.Contains(s, "[") && strings.Contains(s, "]") {
		//ipv6 : for example : [2606:4700:4700::1111]:443

		//strip '[' and ']'
		s = strings.ReplaceAll(s, "[", "")
		s = strings.ReplaceAll(s, "]", "")

		ix = strings.LastIndexAny(s, ":")
		if ix == -1 {
			return s
		}
	} else {
		//ipv4
		ix = strings.IndexRune(s, ':')
		if ix == -1 {
			return s
		}

	}
	return s[:ix]
}

func TLSConfigFromCA(ca *tls.Certificate, proxy *HttpProxy) func(host string) (*tls.Config, error) {
	return func(host string) (*tls.Config, error) {
		var err error
		var cert *tls.Certificate

		hostname := stripPort(host)
		config := defaultTLSConfig.Clone()
		proxy.Debugger("signing for %s", stripPort(host))

		genCert := func() (*tls.Certificate, error) {
			return signHost(*ca, []string{hostname})
		}
		cert, err = genCert()

		if err != nil {
			proxy.Warn("Cannot sign host certificate with provided CA: %s", err)
			return nil, err
		}

		config.Certificates = append(config.Certificates, *cert)
		return config, nil
	}
}

func httpError(w io.WriteCloser, proxy *HttpProxy, err error) {
	errStr := fmt.Sprintf("HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nContent-Length: %d\r\n\r\n%s", len(err.Error()), err.Error())
	if _, err := io.WriteString(w, errStr); err != nil {
		proxy.Warn("Error responding to client: %s", err)
	}
	if err := w.Close(); err != nil {
		proxy.Warn("Error closing client connection: %s", err)
	}
}
