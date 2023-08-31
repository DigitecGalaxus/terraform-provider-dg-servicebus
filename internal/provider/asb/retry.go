package asb

import (
	"context"
	"math"
	"time"

	"github.com/hashicorp/terraform-plugin-log/tflog"
)

func runWithRetryIncrementalBackOff[TResult any](
	ctx context.Context,
	actionMessage string,
	fun func() (TResult, error),
) (TResult, error) {
	var err error
	var res TResult
	for i := 1; i < 5; i++ {
		res, err := fun()
		if err == nil {
			return res, nil
		}

		tflog.Info(ctx, actionMessage+" failed with error "+err.Error()+", retrying")

		backOff := time.Second * time.Duration(math.Pow(2, float64(i)))
		time.Sleep(backOff)
	}

	return res, err
}

func runWithRetryIncrementalBackOffVoid(
	ctx context.Context,
	actionMessage string,
	fun func() error,
) error {
	_, err := runWithRetryIncrementalBackOff(
		ctx,
		actionMessage,
		func() (interface{}, error) {
			return nil, fun()
		},
	)

	return err
}