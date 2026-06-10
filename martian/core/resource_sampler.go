package core

type ResourceSampler interface {
	SystemMemory() (MemInfo, error)
	Rusage() *RusageInfo
	UserProcessCount() (int, error)
	ProcessTreeMemory(pid int, includeParent bool, io map[int]*IoAmount) (ObservedMemory, error)
	ProcessTreeMemoryList(pid int) (ProcessTree, error)
	RunningMemory(pid int) (ObservedMemory, error)
	RunningIo(pid int) (*IoAmount, error)
}

type defaultResourceSampler struct{}

func NewResourceSampler() ResourceSampler {
	return defaultResourceSampler{}
}

func (defaultResourceSampler) SystemMemory() (MemInfo, error) {
	var info MemInfo
	return info, info.Get()
}

func (defaultResourceSampler) Rusage() *RusageInfo {
	return GetRusage()
}

func (defaultResourceSampler) UserProcessCount() (int, error) {
	return GetUserProcessCount()
}

func (defaultResourceSampler) ProcessTreeMemory(
	pid int, includeParent bool, io map[int]*IoAmount) (ObservedMemory, error) {
	return GetProcessTreeMemory(pid, includeParent, io)
}

func (defaultResourceSampler) ProcessTreeMemoryList(pid int) (ProcessTree, error) {
	return GetProcessTreeMemoryList(pid)
}

func (defaultResourceSampler) RunningMemory(pid int) (ObservedMemory, error) {
	return GetRunningMemory(pid)
}

func (defaultResourceSampler) RunningIo(pid int) (*IoAmount, error) {
	return GetRunningIo(pid)
}
