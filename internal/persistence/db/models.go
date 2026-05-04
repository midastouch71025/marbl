package db

type TaskState string

const (
	TaskStateReceived   TaskState = "received"
	TaskStateProcessing TaskState = "processing"
	TaskStateDone       TaskState = "done"
)

type Task struct {
	ID             int64
	Type           int16
	Value          int16
	State          TaskState
	CreationTime   float64
	LastUpdateTime float64
}

type TaskCountsByStateRow struct {
	State TaskState
	Cnt   int64
}
