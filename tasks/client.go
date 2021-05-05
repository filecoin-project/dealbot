package tasks

import "github.com/ipld/go-ipld-prime/schema"

// accessor methods for working with client structs
func (ptp *_PopTask__Prototype) Of(workedBy string, status Status) *_PopTask {
	pt := _PopTask{
		WorkedBy: _String{workedBy},
		Status:   *status,
	}
	return &pt
}

func (utp *_UpdateTask__Prototype) Of(workedBy string, status Status) *_UpdateTask {
	ut := _UpdateTask{
		WorkedBy:            _String{workedBy},
		Status:              *status,
		CurrentStageDetails: _StageDetails__Maybe{m: schema.Maybe_Absent},
		Stage:               _String__Maybe{m: schema.Maybe_Absent},
	}
	return &ut
}

func (utp *_UpdateTask__Prototype) OfStage(workedBy string, status Status, stage string, details StageDetails) *_UpdateTask {
	ut := _UpdateTask{
		WorkedBy:            _String{workedBy},
		Status:              *status,
		CurrentStageDetails: _StageDetails__Maybe{m: schema.Maybe_Value, v: details},
		Stage:               _String__Maybe{m: schema.Maybe_Value, v: &_String{stage}},
	}
	return &ut
}
