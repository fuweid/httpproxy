package proxy

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"
)

var (
	defaultTimeout = 30 * time.Second
	maxIdleConns   = 100
)

type proxyServer struct {
	transport *http.Transport
}

func NewProxyServer() http.HandlerFunc {
	return (&proxyServer{
		transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   defaultTimeout,
				KeepAlive: defaultTimeout,
			}).DialContext,
			ForceAttemptHTTP2:     false,
			MaxIdleConns:          maxIdleConns,
			IdleConnTimeout:       defaultTimeout,
			TLSHandshakeTimeout:   defaultTimeout,
			ExpectContinueTimeout: defaultTimeout,
		},
	}).serveHTTP
}

func (ps *proxyServer) serveHTTP(respW http.ResponseWriter, req *http.Request) {
	ctx := withRequestID(req.Context())
	req = req.WithContext(ctx)

	switch req.Method {
	case "CONNECT":
		log(req).Info("Tunnel")
		ps.handleTunnel(respW, req)
	default:
		log(req).Info("Proxy")
		ps.handleHTTP(respW, req)
	}
}

func (ps *proxyServer) handleHTTP(respW http.ResponseWriter, req *http.Request) {
	fresp, err := ps.transport.RoundTrip(req)
	if err != nil {
		log(req).Errorf("failed to handle: %v", err)

		http.Error(respW, err.Error(), http.StatusBadGateway)
		return
	}
	defer fresp.Body.Close()

	copyResponseHeader(respW, fresp.Header)
	respW.WriteHeader(fresp.StatusCode)
	if _, err := io.Copy(respW, fresp.Body); err != nil {
		log(req).Errorf("failed to forward response: %v", err)
	}
}

func (ps *proxyServer) handleTunnel(respW http.ResponseWriter, req *http.Request) {
	dctx, dcancel := context.WithTimeout(req.Context(), defaultTimeout)
	defer dcancel()

	remoteConn, err := (&net.Dialer{}).DialContext(dctx, "tcp", req.Host)
	if err != nil {
		log(req).Errorf("failed to connect remote host %v: %v", req.Host, err)

		http.Error(respW, err.Error(), http.StatusBadGateway)
		return
	}

	respW.WriteHeader(http.StatusOK)

	hijacker, ok := respW.(http.Hijacker)
	if !ok {
		log(req).Error("Hijacking not supported")

		http.Error(respW, "Hijacking not supported", http.StatusBadGateway)
		return
	}

	localConn, localReader, err := hijacker.Hijack()
	if err != nil {
		log(req).Errorf("failed to hijack: %v", err)

		http.Error(respW, err.Error(), http.StatusBadGateway)
		return
	}

	defer func() {
		remoteConn.Close()
		localConn.Close()
	}()
	go io.Copy(remoteConn, localReader)

	if _, err := io.Copy(localConn, remoteConn); err != nil {
		log(req).Errorf("failed to forward data: %v", err)
	}
}

func copyResponseHeader(respW http.ResponseWriter, source http.Header) {
	for key, vals := range source {
		for _, val := range vals {
			respW.Header().Add(key, val)
		}
	}
}
