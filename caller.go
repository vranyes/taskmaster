package taskmaster

import "context"

type DownstreamCaller interface {
	PerformTask(ctx context.Context, task string) (string, error)
}

type FakeCaller struct {
	Result            string
	Err               error
	BlockUntilCtxDone bool
	SawDeadline       bool
}

func (f *FakeCaller) PerformTask(ctx context.Context, task string) (string, error) {
	_, f.SawDeadline = ctx.Deadline()
	if f.BlockUntilCtxDone {
		<-ctx.Done()
		return "", ctx.Err()
	}
	if f.Err != nil {
		return "", f.Err
	}
	return f.Result, nil
}
