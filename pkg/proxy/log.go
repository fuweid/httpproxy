package proxy

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

type requestIDKey struct{}

func withRequestID(ctx context.Context) context.Context {
	var b [8]byte
	rand.Read(b[:])

	return context.WithValue(ctx, requestIDKey{}, fmt.Sprintf("%s", base64.URLEncoding.EncodeToString(b[:])))
}

func requestIDFrom(ctx context.Context) string {
	return ctx.Value(requestIDKey{}).(string)
}

func log(req *http.Request) *logrus.Entry {
	return logrus.
		WithField("Request-ID", requestIDFrom(req.Context())).
		WithField("Method", req.Method).
		WithField("Proto", req.Proto).
		WithField("RequestUrl", req.RequestURI)
}
