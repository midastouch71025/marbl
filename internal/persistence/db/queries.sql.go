package db

import "context"

type CreateTaskParams struct {
	Type  int16
	Value int16
}

func (q *Queries) CreateTask(ctx context.Context, arg CreateTaskParams) (Task, error) {
	const sql = `INSERT INTO tasks (type, value, state, creation_time, last_update_time)
VALUES ($1, $2, 'received', extract(epoch from now()), extract(epoch from now()))
RETURNING id, type, value, state, creation_time, last_update_time`
	row := q.db.QueryRow(ctx, sql, arg.Type, arg.Value)
	var t Task
	if err := row.Scan(
		&t.ID,
		&t.Type,
		&t.Value,
		&t.State,
		&t.CreationTime,
		&t.LastUpdateTime,
	); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (q *Queries) GetTask(ctx context.Context, id int64) (Task, error) {
	const sql = `SELECT id, type, value, state, creation_time, last_update_time
FROM tasks
WHERE id = $1`
	row := q.db.QueryRow(ctx, sql, id)
	var t Task
	if err := row.Scan(
		&t.ID,
		&t.Type,
		&t.Value,
		&t.State,
		&t.CreationTime,
		&t.LastUpdateTime,
	); err != nil {
		return Task{}, err
	}
	return t, nil
}

func (q *Queries) TryMarkProcessing(ctx context.Context, id int64) (int64, error) {
	const sql = `UPDATE tasks
SET state = 'processing', last_update_time = extract(epoch from now())
WHERE id = $1 AND state = 'received'`
	tag, err := q.db.Exec(ctx, sql, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (q *Queries) MarkDone(ctx context.Context, id int64) (int64, error) {
	const sql = `UPDATE tasks
SET state = 'done', last_update_time = extract(epoch from now())
WHERE id = $1 AND state = 'processing'`
	tag, err := q.db.Exec(ctx, sql, id)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (q *Queries) CountBacklog(ctx context.Context) (int64, error) {
	const sql = `SELECT COUNT(*)::bigint
FROM tasks
WHERE state IN ('received', 'processing')`
	row := q.db.QueryRow(ctx, sql)
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (q *Queries) ListReceivedTasks(ctx context.Context, limit int32) ([]Task, error) {
	const sql = `SELECT id, type, value, state, creation_time, last_update_time
FROM tasks
WHERE state = 'received'
ORDER BY id ASC
LIMIT $1`
	rows, err := q.db.Query(ctx, sql, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Task
	for rows.Next() {
		var t Task
		if err := rows.Scan(
			&t.ID,
			&t.Type,
			&t.Value,
			&t.State,
			&t.CreationTime,
			&t.LastUpdateTime,
		); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (q *Queries) ReapStaleProcessing(ctx context.Context, staleBefore float64) (int64, error) {
	const sql = `UPDATE tasks
SET state = 'received', last_update_time = extract(epoch from now())
WHERE state = 'processing' AND last_update_time < $1`
	tag, err := q.db.Exec(ctx, sql, staleBefore)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (q *Queries) TaskCountsByState(ctx context.Context) ([]TaskCountsByStateRow, error) {
	const sql = `SELECT state, COUNT(*)::bigint AS cnt
FROM tasks
GROUP BY state`
	rows, err := q.db.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TaskCountsByStateRow
	for rows.Next() {
		var r TaskCountsByStateRow
		if err := rows.Scan(&r.State, &r.Cnt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (q *Queries) SumValueDoneByType(ctx context.Context, typeArg int16) (int64, error) {
	const sql = `SELECT COALESCE(SUM(value), 0)::bigint AS total
FROM tasks
WHERE state = 'done' AND type = $1`
	row := q.db.QueryRow(ctx, sql, typeArg)
	var total int64
	if err := row.Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}

func (q *Queries) CountDoneByType(ctx context.Context, typeArg int16) (int64, error) {
	const sql = `SELECT COUNT(*)::bigint
FROM tasks
WHERE state = 'done' AND type = $1`
	row := q.db.QueryRow(ctx, sql, typeArg)
	var n int64
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}

func (q *Queries) SqlcToolingPing(ctx context.Context) (int32, error) {
	const sql = `SELECT 1::int AS n`
	row := q.db.QueryRow(ctx, sql)
	var n int32
	if err := row.Scan(&n); err != nil {
		return 0, err
	}
	return n, nil
}
