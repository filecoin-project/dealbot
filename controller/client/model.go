package client

import "github.com/filecoin-project/dealbot/tasks"

type UpdateTaskRequest struct {
	Status   tasks.Status `json:"status"`
	WorkedBy string       `json:"worked_by,omitempty"` // which dealbot works on that task
}

type PopTaskRequest struct {
	WorkedBy string
}
