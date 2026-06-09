package resource

type ProgressObserver interface {
	OnProgress(taskName string, percentage float64, status string, category string)
}

type NopProgressObserver struct{}

func (n *NopProgressObserver) OnProgress(taskName string, percentage float64, status string, category string) {
}
