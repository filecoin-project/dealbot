package state

type record_update_status int

const (
	LATEST_UPDATE     record_update_status = 1
	PREVIOUS_UPDATE   record_update_status = 2
	UNATTACHED_RECORD record_update_status = 10
	ATTACHED_RECORD   record_update_status = 11
)
