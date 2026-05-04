package db

import "context"

type Querier interface {
	CreateTask(ctx context.Context, arg CreateTaskParams) (Task, error)
	GetTask(ctx context.Context, id int64) (Task, error)
	TryMarkProcessing(ctx context.Context, id int64) (int64, error)
	MarkDone(ctx context.Context, id int64) (int64, error)
	CountBacklog(ctx context.Context) (int64, error)
	ListReceivedTasks(ctx context.Context, limit int32) ([]Task, error)
	ReapStaleProcessing(ctx context.Context, staleBefore float64) (int64, error)
	TaskCountsByState(ctx context.Context) ([]TaskCountsByStateRow, error)
	SumValueDoneByType(ctx context.Context, typeArg int16) (int64, error)
	CountDoneByType(ctx context.Context, typeArg int16) (int64, error)
	SqlcToolingPing(ctx context.Context) (int32, error)
}

var _ Querier = (*Queries)(nil)
