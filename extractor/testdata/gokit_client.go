package sample

import (
	"time"

	"github.com/go-kit/kit/sd/lb"
)

func MakeEndpoint() {
	lb.Retry(3, 5*time.Second, nil)
}
