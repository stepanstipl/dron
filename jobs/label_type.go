package jobs

//go:generate enumer -type=LabelType
type LabelType uint8

const ( unknown LabelType = iota
	command
	schedule
)