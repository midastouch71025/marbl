-- name: CreateTask :one
INSERT INTO tasks (type, value, state, creation_time, last_update_time)
VALUES ($1, $2, 'received', extract(epoch from now()), extract(epoch from now()))
RETURNING id, type, value, state, creation_time, last_update_time;

-- name: GetTask :one
SELECT id, type, value, state, creation_time, last_update_time
FROM tasks
WHERE id = $1;

-- name: TryMarkProcessing :execrows
UPDATE tasks
SET state = 'processing', last_update_time = extract(epoch from now())
WHERE id = $1 AND state = 'received';

-- name: MarkDone :execrows
UPDATE tasks
SET state = 'done', last_update_time = extract(epoch from now())
WHERE id = $1 AND state = 'processing';

-- name: CountBacklog :one
SELECT COUNT(*)::bigint
FROM tasks
WHERE state IN ('received', 'processing');

-- name: ListReceivedTasks :many
SELECT id, type, value, state, creation_time, last_update_time
FROM tasks
WHERE state = 'received'
ORDER BY id ASC
LIMIT $1;

-- name: ReapStaleProcessing :execrows
UPDATE tasks
SET state = 'received', last_update_time = extract(epoch from now())
WHERE state = 'processing' AND last_update_time < $1;

-- name: TaskCountsByState :many
SELECT state, COUNT(*)::bigint AS cnt
FROM tasks
GROUP BY state;

-- name: SumValueDoneByType :one
SELECT COALESCE(SUM(value), 0)::bigint AS total
FROM tasks
WHERE state = 'done' AND type = $1;

-- name: CountDoneByType :one
SELECT COUNT(*)::bigint
FROM tasks
WHERE state = 'done' AND type = $1;

-- name: SqlcToolingPing :one
SELECT 1::int AS n;
