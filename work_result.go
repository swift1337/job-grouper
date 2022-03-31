package jobgrouper

type JobStatus int

const (
	Pending JobStatus = iota
	Running
	Finished
	Error
	Panicked
	Aborted
)

type JobResult struct {
	Error  error
	Status JobStatus
}

var Statuses = map[JobStatus]string{
	Pending:  "pending",
	Running:  "running",
	Finished: "finished",
	Error:    "error",
	Panicked: "panicked",
	Aborted:  "aborted",
}

func (jr JobResult) StatusName() string {
	return Statuses[jr.Status]
}
