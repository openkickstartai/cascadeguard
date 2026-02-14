package sample

import (
	"context"
	"net/http"
	"time"

	"google.golang.org/grpc"
	"github.com/avast/retry-go"
)

func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 5 * time.Second,
	}
}

func CallWithContext(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_ = ctx
}

func DialGRPC() {
	grpc.WithTimeout(10 * time.Second)
}

func DoWithRetry() {
	retry.Do(
		func() error { return nil },
		retry.Attempts(3),
	)
}
