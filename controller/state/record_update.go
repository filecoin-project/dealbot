package state

type RecordUpdateStatus int

const (
	LATEST_UPDATE     RecordUpdateStatus = 1
	PREVIOUS_UPDATE   RecordUpdateStatus = 2
	UNATTACHED_RECORD RecordUpdateStatus = 10
	ATTACHED_RECORD   RecordUpdateStatus = 11
)
