package state

// Define state database DML.  Any DDL is located in migrations.
const (
	assignTaskSQL = `
		UPDATE tasks SET data = $2, worked_by = $3, cid = $4
		WHERE uuid = $1
	`

	createTaskSQL = `
		INSERT INTO tasks (uuid, data, created, cid, tag) VALUES($1, $2, $3, $4, $5)
	`

	setTaskStatusSQL = `
		INSERT INTO task_status_ledger (uuid, status, stage, ts) VALUES($1, $2, '', $3)
	`

	upsertTaskStatusSQL = `
		INSERT INTO task_status_ledger (uuid, status, stage, run, ts) VALUES($1, $2, $3, $4, $5)
	`

	updateTaskDataSQL = `
		UPDATE tasks SET data = $2, cid = $3 WHERE uuid = $1
	`

	countAllTasksSQL = `
		SELECT COUNT(1) FROM tasks
	`

	getAllTasksSQL = `
		SELECT data FROM tasks
	`

	getTaskSQL = `
		SELECT data FROM tasks WHERE uuid = $1
	`

	getTaskByCidSQL = `
		SELECT data FROM tasks WHERE cid = $1
	`

	currentTaskStatusSQL = `
		SELECT status, stage FROM task_status_ledger WHERE uuid = $1 ORDER BY ts DESC limit 1
	`

	oldestAvailableTaskSQL = `
		SELECT uuid, data FROM tasks
		WHERE worked_by IS NULL
		ORDER BY created
		LIMIT 1
	`

	oldestAvailableTaskWithTagsSQL = `
		SELECT uuid, data FROM tasks
		WHERE worked_by IS NULL
		AND (tag is NULL OR tag = ANY($1))
		ORDER BY created
		LIMIT 1
	`

	oldestAvailableTaskWithTagsSQLsqlite = `
		SELECT uuid, data FROM tasks
		WHERE worked_by IS NULL
		AND (tag is NULL OR tag IN (%s))
		ORDER BY created
		LIMIT 1
	`

	taskHistorySQL = `
		SELECT status, stage, run, ts FROM task_status_ledger WHERE uuid = $1 ORDER BY ts
	`

	cidArchiveSQL = `
		INSERT INTO finalizedData (cid, data, created) VALUES($1, $2, $3) ON CONFLICT (cid) DO NOTHING
	`

	cidGetArchiveSQL = `
		SELECT data FROM finalizedData WHERE cid = $1
	`

	drainedAddSQL = `
		INSERT INTO drained_workers (worked_by) VALUES ($1)
	`

	drainedQuerySQL = `
		SELECT COUNT(*) as cnt FROM drained_workers WHERE worked_by = $1
	`

	addHeadSQL = `
		INSERT INTO record_updates (cid, created, worked_by, status) VALUES ($1, $2, $3, $4)
	`

	updateHeadSQL = `
		UPDATE record_updates SET status = $1 WHERE cid = $2
	`

	queryHeadSQL = `
		SELECT cid FROM record_updates WHERE status = $1 AND worked_by = $2
	`

	workerTasksByStatusSQL = `
	SELECT tasks.data FROM tasks
	INNER JOIN task_status_ledger ON tasks.uuid=task_status_ledger.uuid
	WHERE tasks.worked_by = $1 AND task_status_ledger.status = $2
`

	unassignTaskSQL = `
		UPDATE tasks SET data = $2, worked_by = NULL, cid = $3
		WHERE uuid = $1
	`
)
