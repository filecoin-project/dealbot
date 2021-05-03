package client

import "github.com/filecoin-project/dealbot/tasks"

// PopTaskRequest specifies parameters to pop the next assigned task
type PopTaskRequest struct {
	// Status indicates the status the task should be in after assignment
	Status tasks.Status `json:"status"`
	// WorkedBy is the dealbot that will be assigned to the task
	WorkedBy string `json:"worked_by,omitempty"`
}

// UpdateTaskRequest specifies parameters for updating a task
type UpdateTaskRequest struct {
	// Status is the global task status, shared among all tasks types -- always one of Available, InProgress, Successful, or Failure
	Status tasks.Status `json:"status"`
	// Stage is a more detailed identifier for what part of the process the deal is in. Some stages are unique to the task type
	// When a task status is Successful or Failure, Stage indicates the final stage of the deal making process that was reached
	Stage string
	// CurrentStageDeatails offers more information about what is happening in the current stage of the deal making process
	CurrentStageDetails tasks.StageDetails
	// WorkedBy indicates the dealbot making this update request
	WorkedBy string `json:"worked_by,omitempty"`
}
