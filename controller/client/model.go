package client

import "github.com/filecoin-project/dealbot/tasks"

type PopTaskRequest struct {
	Status   tasks.Status `json:"status"`
	WorkedBy string       `json:"worked_by,omitempty"` // which dealbot works on that task
}

type UpdateTaskRequest struct {
	Status    tasks.Status `json:"status"`
	StageName string
	StageData *tasks.StageData
	WorkedBy  string `json:"worked_by,omitempty"` // which dealbot works on that task
}
