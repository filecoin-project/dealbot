package tasks

// accessor methods for working with client structs
//func (ptp *_PopTask__Prototype) Of(workedBy string, status Status, tags ...string) *_PopTask {
//	s := make([]_String, len(tags))
//	for i := range tags {
//		s[i] = _String{tags[i]}
//	}
//	ls := _List_String{
//		x: s,
//	}
//
//	pt := _PopTask{
//		WorkedBy: _String{workedBy},
//		Status:   *status,
//		Tags:     _List_String__Maybe{m: schema.Maybe_Value, v: ls},
//	}
//	return &pt
//}

func NewPopTask(workedBy string, status Status, tags ...string) *PopTask {
	s := make([]string, len(tags))
	for i := range tags {
		s[i] = tags[i]
	}

	pt := PopTask{
		WorkedBy: workedBy,
		Status:   status,
		Tags:     s,
	}
	return &pt
}

//func (utp *_UpdateTask__Prototype) Of(workedBy string, status Status, runCount int) *_UpdateTask {
//	ut := _UpdateTask{
//		WorkedBy:            _String{workedBy},
//		Status:              *status,
//		ErrorMessage:        _String__Maybe{m: schema.Maybe_Absent},
//		CurrentStageDetails: _StageDetails__Maybe{m: schema.Maybe_Absent},
//		Stage:               _String__Maybe{m: schema.Maybe_Absent},
//		RunCount:            _Int{int64(runCount)},
//	}
//	return &ut
//}

func NewUpdateTask(workedBy string, status Status, runCount int) *UpdateTask {
	ut := UpdateTask{
		WorkedBy:            workedBy,
		Status:              status,
		ErrorMessage:        nil,
		CurrentStageDetails: nil,
		Stage:               nil,
		RunCount:            int64(runCount),
	}
	return &ut
}

func NewUpdateTaskWithStage(workedBy string, status Status, errorMessage string, stage string, details *StageDetails, runCount int) *UpdateTask {
	var err *string
	if errorMessage == "" {
		err = nil
	} else {
		err = &errorMessage
	}
	ut := UpdateTask{
		WorkedBy:            workedBy,
		Status:              status,
		ErrorMessage:        err,
		CurrentStageDetails: details,
		Stage:               &stage,
		RunCount:            int64(runCount),
	}
	return &ut
}

//func (utp *_UpdateTask__Prototype) OfStage(workedBy string, status Status, errorMessage string, stage string, details StageDetails, runCount int) *_UpdateTask {
//	var err _String__Maybe
//	if errorMessage == "" {
//		err = _String__Maybe{m: schema.Maybe_Absent}
//	} else {
//		err = _String__Maybe{m: schema.Maybe_Value, v: _String{errorMessage}}
//	}
//	ut := _UpdateTask{
//		WorkedBy:            _String{workedBy},
//		Status:              *status,
//		ErrorMessage:        err,
//		CurrentStageDetails: _StageDetails__Maybe{m: schema.Maybe_Value, v: details},
//		Stage:               _String__Maybe{m: schema.Maybe_Value, v: _String{stage}},
//		RunCount:            _Int{int64(runCount)},
//	}
//	return &ut
//}
