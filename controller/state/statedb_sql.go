package state

const (
	createTasksTableSQL = `
		CREATE TABLE IF NOT EXISTS tasks (
			uuid varchar(36) NOT NULL,
			worked_by text,
			data text NOT NULL CHECK (data != ''),
			created timestamp NOT NULL,
			PRIMARY KEY(uuid)
		)
	`

	createStatusLedgerSQL = `
		CREATE TABLE IF NOT EXISTS task_status_ledger (
			uuid varchar(36) NOT NULL,
			status int,
			ts timestamp NOT NULL,
			PRIMARY KEY(uuid, ts)
			CONSTRAINT fk_status_ledger_uuid FOREIGN KEY (uuid) REFERENCES tasks
		)
	`

	assignTaskSQL = `
		UPDATE tasks SET data = $2, worked_by = $3
		WHERE uuid = $1
	`

	createTaskSQL = `
		INSERT INTO tasks (uuid, data, created) VALUES($1, $2, $3)
	`

	setTaskStatusSQL = `
		INSERT INTO task_status_ledger (uuid, status, ts) VALUES($1, $2, $3)
	`

	updateTaskDataSQL = `
		UPDATE tasks SET data = $2 WHERE uuid = $1
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

	currentTaskStatusSQL = `
		SELECT status FROM task_status_ledger WHERE uuid = $1 ORDER BY ts DESC limit 1
	`

	oldestAvailableTaskSQL = `
		SELECT uuid, data FROM tasks
		WHERE worked_by IS NULL
		ORDER BY created
		LIMIT 1
	`

	taskHistorySQL = `
		SELECT status, ts FROM task_status_ledger WHERE uuid = $1 ORDER BY ts
	`
)
