package controller

const (
	createTasksTable = `
		CREATE TABLE IF NOT EXISTS tasks (
			uuid text PRIMARY KEY,
			data text NOT NULL CHECK (data != ''),
			created_at timestamp NOT NULL,
			updated_at timestamp NOT NULL
		);
	`

	countTasksSql = `
		SELECT COUNT(*) FROM tasks
	`

	getAllTasks = `
		SELECT * FROM tasks ORDER BY updated_at DESC
	`

	getTask = `
		SELECT data FROM tasks WHERE uuid = $1 limit 1 
	`

	insertTask = `
		INSERT INTO tasks(uuid, data, created_at, updated_at) VALUES($1, $2, $3, $3)
	`

	updateTask = `
		UPDATE tasks SET data = $2, updated_at = $3 WHERE uuid = $1
	`
)
