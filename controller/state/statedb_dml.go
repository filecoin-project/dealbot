package state

// Define state database DML.  Any DDL is located in migrations.
const (
	assignTaskSQL = `
		UPDATE tasks SET data = $2, worked_by = $3, cid = $4
		WHERE uuid = $1
	`

	createTaskSQL = `
		INSERT INTO tasks (uuid, data, created, cid) VALUES($1, $2, $3, $4)
	`

	setTaskStatusSQL = `
		INSERT INTO task_status_ledger (uuid, status, stage, ts) VALUES($1, $2, '', $3)
	`

	upsertTaskStatusSQL = `
		INSERT INTO task_status_ledger (uuid, status, stage, ts) VALUES($1, $2, $3, $4)
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

	taskHistorySQL = `
		SELECT status, stage, ts FROM task_status_ledger WHERE uuid = $1 ORDER BY ts
	`

	cidArchiveSQL = `
		INSERT INTO finalizedData (cid, data, created) VALUES($1, $2, $3)
	`
)
