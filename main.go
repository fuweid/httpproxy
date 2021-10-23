package main

import (
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

var dialTimeout = 30 * time.Second

func main() {
	server := &http.Server{
		Addr:    ":8080",
		Handler: http.HandlerFunc(newProxyServer().serveHTTP),
	}
	log.Fatal(server.ListenAndServe())
}

type proxyServer struct {
	transport *http.Transport
}

func newProxyServer() *proxyServer {
	return &proxyServer{
		transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
}

func (ps *proxyServer) serveHTTP(respW http.ResponseWriter, req *http.Request) {
	if req.Method == "CONNECT" {
		ps.handleConnectTunnel(respW, req)
		return
	}
	ps.handleHTTP(respW, req)
}

func (ps *proxyServer) handleHTTP(respW http.ResponseWriter, req *http.Request) {
	fresp, err := ps.transport.RoundTrip(req)
	if err != nil {
		logError("failed to forward request: %v", err)

		http.Error(respW, err.Error(), http.StatusBadGateway)
		return
	}
	defer fresp.Body.Close()

	copyResponseHeader(respW, fresp.Header)
	respW.WriteHeader(fresp.StatusCode)
	if _, err := io.Copy(respW, fresp.Body); err != nil {
		logError("failed to forward response: %v", err)
	}
}

func (ps *proxyServer) handleConnectTunnel(respW http.ResponseWriter, req *http.Request) {
	remoteConn, err := net.DialTimeout("tcp", req.Host, dialTimeout)
	if err != nil {
		http.Error(respW, err.Error(), http.StatusBadGateway)
		return
	}

	respW.WriteHeader(http.StatusOK)
	hijacker, ok := respW.(http.Hijacker)
	if !ok {
		http.Error(respW, "Hijacking not supported", http.StatusBadGateway)
		return
	}

	localConn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(respW, err.Error(), http.StatusBadGateway)
		return
	}

	go func() {
		defer func() {
			localConn.Close()
			remoteConn.Close()
		}()
		io.Copy(remoteConn, localConn)
	}()

	go func() {
		defer func() {
			remoteConn.Close()
			localConn.Close()
		}()
		io.Copy(localConn, remoteConn)
	}()
}

func copyResponseHeader(respW http.ResponseWriter, source http.Header) {
	for key, vals := range source {
		for _, val := range vals {
			respW.Header().Add(key, val)
		}
	}
}

func logError(format string, args ...interface{}) {
	log.Printf(format, args...)
}
