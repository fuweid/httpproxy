package proxy

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

var (
	defaultBufSize = 1 << 12

	bufPool = sync.Pool{
		New: func() interface{} {
			buffer := make([]byte, defaultBufSize)
			return &buffer
		},
	}
)

type LimitRule struct {
	LimitedBytesPerSec int
	RetryAfter         time.Duration
}

func (lr LimitRule) Valid() bool {
	return lr.LimitedBytesPerSec >= defaultBufSize
}

type ioLimiter struct {
	rate *rate.Limiter
	lr   LimitRule
}

func newIOLimiter(lr LimitRule) *ioLimiter {
	if !lr.Valid() {
		return nil
	}

	bucketSize := lr.LimitedBytesPerSec
	return &ioLimiter{
		rate: rate.NewLimiter(
			rate.Limit(bucketSize),
			bucketSize+defaultBufSize, // add burst
		),
		lr: lr,
	}
}

func (l *ioLimiter) acquireN(n int) time.Duration {
	if l == nil {
		return 0
	}

	return l.rate.ReserveN(time.Now(), n).Delay()
}

func (l *ioLimiter) hitTooManyBytes(n int) bool {
	if l == nil {
		return false
	}

	return !l.rate.AllowN(time.Now(), n)
}

func (l *ioLimiter) retryAfter() time.Duration {
	if l == nil {
		return 0
	}
	return l.lr.RetryAfter
}

func (ps *proxyServer) copyWithLimiter(ctx context.Context, logger *logrus.Entry, dest io.Writer, src io.Reader) (int64, error) {
	bufRef := bufPool.Get().(*[]byte)
	defer bufPool.Put(bufRef)
	buf := *bufRef

	stopTimer := func(t *time.Timer, recv bool) {
		if !t.Stop() && recv {
			<-t.C
		}
	}

	var written int64
	var timer = time.NewTimer(0)
	stopTimer(timer, true)

	for {
		nr, er := io.ReadAtLeast(src, buf, len(buf))
		if nr > 0 {
			if ps.limiter.hitTooManyBytes(nr) {
				retryAfter := ps.limiter.retryAfter()
				logger.Warnf("hit io limit rule, retry after %v", retryAfter)

				recv := true
				timer.Reset(retryAfter)
				select {
				case <-timer.C:
					recv = false
				case <-ctx.Done():
					return written, ctx.Err()
				}
				stopTimer(timer, recv)
			}

			if delay := ps.limiter.acquireN(nr); delay > 0 {
				recv := true
				timer.Reset(delay)

				select {
				case <-timer.C:
					recv = false
				case <-ctx.Done():
					return written, ctx.Err()
				}
				stopTimer(timer, recv)
			}

			nw, ew := dest.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if ew != nil {
				return written, ew
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if er != nil {
			if er != io.EOF && er != io.ErrUnexpectedEOF {
				return written, er
			}
			break
		}
	}
	return written, nil
}
