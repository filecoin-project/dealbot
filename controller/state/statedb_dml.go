package state

const (
	createTasksTableSql = `
		CREATE TABLE IF NOT EXISTS tasks (
			uuid text,
			data text NOT NULL CHECK (data != ''),
			ts timestamp NOT NULL,
			PRIMARY KEY(uuid, ts)
		)
	`

	countTasksSql = `
		SELECT COUNT(1) FROM tasks
	`

	getAllTasksSql = `
		SELECT data FROM tasks ORDER BY updated_at DESC
	`

	getLatestTasksSql = `
		SELECT data FROM tasks t1
		WHERE ts=(SELECT MAX(ts) FROM tasks t2 WHERE t1.uuid = t2.uuid)
	`

	getTaskSql = `
		SELECT data FROM tasks WHERE uuid = $1 ORDER BY ts DESC limit 1
	`

	insertTaskSql = `
		INSERT INTO tasks (uuid, data, ts) VALUES($1, $2, $3)
	`
)
