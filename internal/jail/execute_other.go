//go:build !linux
// +build !linux

package jail

import (
	"context"
	"errors"
)

func Execute(ctx context.Context, nsjailBin string, job Job) (Outcome, error) {
	return Outcome{}, errors.New("jail.Execute requires linux; run inside the container")
}
