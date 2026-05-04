package consumer

import (
	"context"

	"marbl/internal/persistence/db"
)

// TaskStore is the DB surface used by the HTTP task handler (kept narrow for tests).
type TaskStore interface {
	GetTask(ctx context.Context, id int64) (db.Task, error)
	TryMarkProcessing(ctx context.Context, id int64) (int64, error)
	MarkDone(ctx context.Context, id int64) (int64, error)
	SumValueDoneByType(ctx context.Context, typeArg int16) (int64, error)
	CountDoneByType(ctx context.Context, typeArg int16) (int64, error)
}

var _ TaskStore = (*db.Queries)(nil)
