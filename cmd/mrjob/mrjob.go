//
// Copyright (c) 2020 10X Genomics, Inc. All rights reserved.
//

// Command mrjob manages process lifetimes for Martian stage code.
//
// Also collects various performance statistics.
package main

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"runtime/pprof"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/google/shlex"

	"github.com/martian-lang/martian/martian/core"
	"github.com/martian-lang/martian/martian/util"
)

const HeartbeatInterval = time.Minute * 2
const MemorySampleInterval = time.Second * 5

// Sample memory usage with much higher frequency when monitoring.
const MonitorMemorySampleInterval = time.Second * 1

type runner struct {
	start      time.Time
	job        *exec.Cmd
	log        *os.File
	control    controlChannel
	ioStats    *core.IoStatsBuilder
	metadata   *core.Metadata
	jobInfo    *core.JobInfo
	supervisor processSupervisor
	isDone     chan struct{}
	perfDone   <-chan struct{}
	runType    string
	highMem    core.ObservedMemory
	monitoring bool
	sampler    core.ResourceSampler
}

func main() {
	util.SetupSignalHandlers()
	if len(os.Args) < 6 {
		panic("Insufficient arguments.\n" +
			"Expected: mrjob <exe> [exe args...] <split|main|join> " +
			"<metadata_path> <files_path> <journal_prefix>")
	}
	args := os.Args[len(os.Args)-4:]
	runType := args[0]
	metadataPath := args[1]
	filesPath := args[2]
	fqname := path.Base(args[3])
	journalPath := path.Dir(args[3])

	if os.Getenv("MRO_SELF_PROFILE") != "" {
		startCpuProfile(metadataPath)
	}

	run := runner{
		ioStats:    core.NewIoStatsBuilder(),
		metadata:   core.NewMetadataRunWithJournalPath(fqname, metadataPath, filesPath, journalPath, runType),
		supervisor: newProcessSupervisor(),
		sampler:    core.NewResourceSampler(),
		runType:    runType,
		start:      time.Now(),
	}
	util.RegisterSignalHandler(&run)
	if log, err := os.OpenFile(run.metadata.MetadataFilePath(core.LogFile),
		os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644); err != nil {
		run.Fail(err, "Can't open log file.")
	} else {
		run.log = log
		util.LogTeeWriter(log)
		defer run.log.Close()
	}
	run.metadata.UpdateJournal(core.StdOut)
	run.metadata.UpdateJournal(core.StdErr)

	run.Init()
	if err := run.StartJob(os.Args[1:]); err != nil {
		run.Fail(err, "Error starting job.")
	}
	run.isDone = make(chan struct{})
	run.WaitLoop()
}

// If we're running a CPU self-profile, this is the handle to it.
var selfProfile *os.File

func startCpuProfile(metadataPath string) {
	// This isn't going through the  usual metadata API because we want
	// the profile to include construction of that.
	f, err := os.Create(path.Join(metadataPath, "_selfProfile.pprof"))
	if err != nil {
		util.PrintError(err, "profile", "Error recording CPU profile")
		return
	}
	if err := pprof.StartCPUProfile(f); err != nil {
		f.Close()
		util.PrintError(err, "profile", "Error recording CPU profile")
		return
	}
	selfProfile = f
}

func (self *runner) Init() {
	// In case the job template was wrong, set the working directory now.
	if err := os.Chdir(self.metadata.FilesPath()); err != nil {
		self.Fail(err, "Could not change to the correct working directory")
	}
	self.writeJobinfo()
	util.LogInfo("time", "__start__")
	if jErr := self.metadata.UpdateJournal(core.LogFile); jErr != nil {
		util.PrintError(jErr, "monitor",
			"Could not update log journal file.  Continuing, hoping for the best.")
	}
	if core.ShouldSetVMemRLimit(self.jobInfo) {
		core.CheckMaxVmem(
			uint64(self.jobInfo.VMemGB*1024*1024) * 1024)
	}
	self.setRlimit()
	if cgLim, cgSoftLim, _ := util.GetCgroupMemoryLimit(); cgLim > 0 {
		if cgLim < int64(math.Ceil(self.jobInfo.MemGB*(1024*1024*1024))) {
			util.LogInfo("monitor",
				"WARNING: cgroup memory limit of %d bytes is less than the requested %g GB",
				cgLim, self.jobInfo.MemGB)
		} else {
			util.LogInfo("monitor",
				"cgroup memory limit of %d bytes detected", cgLim)
		}
		if cgSoftLim != 0 && cgSoftLim < int64(math.Ceil(self.jobInfo.MemGB*1024*1024*1024)) {
			util.LogInfo("monitor",
				"WARNING: cgroup soft memory limit of %d bytes is less than the requested %g GB",
				cgSoftLim, self.jobInfo.MemGB)
		}
	}
	setSubreaper()
}

func getClusterEnv() map[string]string {
	re := regexp.MustCompile("^(?:EGO|SGE|LS[BF]|PBS|SLURM|JOB|INSTANCE)_[^O]")
	captures := make(map[string]string)
	for _, env := range os.Environ() {
		sep := strings.Index(env, "=")
		if sep > 0 && re.MatchString(env[:sep]) {
			captures[env[:sep]] = env[sep+1:]
		}
	}
	if len(captures) == 0 {
		return nil
	} else {
		return captures
	}
}

func (self *runner) writeJobinfo() {
	jobInfo := new(core.JobInfo)
	if err := self.metadata.ReadInto(core.JobInfoFile, jobInfo); err != nil {
		self.Fail(err, "Error reading jobInfo.")
	} else {
		self.jobInfo = jobInfo
		self.monitoring = jobInfo.Monitor == "monitor"
	}
	self.jobInfo.Cwd = self.metadata.FilesPath()
	self.jobInfo.Host, _ = os.Hostname()
	self.jobInfo.Pid = os.Getpid()
	self.jobInfo.ClusterEnv = getClusterEnv()
	if err := self.metadata.WriteAtomic(core.JobInfoFile, self.jobInfo); err != nil {
		self.Fail(err, "Could not write updated jobInfo.")
	}
}

func (self *runner) setRlimit() {
	if err := core.MaximizeMaxFiles(); err != nil {
		util.PrintError(err, "monitor", "Error setting the file rlimit.")
		return
	}
}

func (self *runner) done() {
	if self.supervisor != nil {
		defer self.supervisor.close()
	}
	util.LogInfo("time", "__end__")
	// refresh jobInfo if possible, but if we can't that's ok.
	self.metadata.ReadInto(core.JobInfoFile, self.jobInfo)
	if self.jobInfo != nil {
		end := time.Now()
		self.jobInfo.WallClockInfo = &core.WallClockInfo{
			Start:    core.WallClockTime(self.start),
			End:      core.WallClockTime(end),
			Duration: end.Sub(self.start).Seconds(),
		}
		if self.supervisor != nil && self.supervisor.waitChildren() {
			if !self.supervisor.reportChildren() {
				// waitChildren detected that there were remaining child
				// processes, but reportChildren wasn't able to report them for
				// whatever reason.
				util.LogInfo("monitor",
					"Orphaned child processes detected, which did not terminate.")
			}
		}
		if self.supervisor != nil {
			self.jobInfo.RusageInfo = self.supervisor.rusage()
		} else {
			self.jobInfo.RusageInfo = core.GetRusage()
		}
		if !self.highMem.IsZero() {
			self.jobInfo.MemoryUsage = &self.highMem
		}
		if self.supervisor != nil {
			self.jobInfo.SupervisorReason = self.supervisor.terminationReason()
		}
		self.jobInfo.IoStats = &self.ioStats.IoStats
		if err := self.metadata.WriteAtomic(core.JobInfoFile, self.jobInfo); err != nil {
			util.PrintError(err, "monitor", "Could not write final jobInfo.")
		} else {
			self.metadata.UpdateJournal(core.JobInfoFile)
		}
		if exceeded, maxRss := self.exceededMemReservation(); exceeded {
			self.writeMemViolation(maxRss)
		}
	}
}

func (self *runner) resourceSampler() core.ResourceSampler {
	if self.sampler == nil {
		self.sampler = core.NewResourceSampler()
	}
	return self.sampler
}

func (self *runner) Fail(err error, message string) {
	self.done()
	errStr := err.Error()
	target := core.Errors
	if _, ok := err.(*stageReturnedError); !ok {
		errStr = fmt.Sprintf("%s\n\n%s\n", message, err.Error())
		fmt.Fprint(os.Stderr, errStr)
	} else {
		if strings.HasPrefix(errStr, "ASSERT:") {
			errStr = errStr[len("ASSERT:"):]
			target = core.Assert
		}
	}
	if writeError := self.metadata.WriteRaw(target, errStr); writeError != nil {
		util.PrintError(writeError, "monitor", "Could not write errors file.")
	}
	if jErr := self.metadata.UpdateJournal(target); jErr != nil {
		util.PrintError(jErr, "monitor", "Could not update %v journal file.", target)
	}
	self.waitForPerf()
	os.Exit(0)
}

// Wait for up to 15 seconds after the stage code terminates for perf record to
// terminate (if applicable).  Otherwise some cluster managers might kill perf
// as soon as the head process for the job (mrjob, in this case) terminates.
func (self *runner) waitForPerf() {
	if c := self.perfDone; c != nil {
		select {
		case <-c:
		case <-time.After(15 * time.Second):
		}
	}
	if selfProfile != nil {
		pprof.StopCPUProfile()
		if err := selfProfile.Close(); err != nil {
			util.PrintError(err, "profile", "Error closing cpu profile")
		}
	}
}

func totalCpu(ru *core.RusageInfo) float64 {
	if ru == nil {
		return 0
	}
	var total float64
	if ru.Self != nil {
		total += ru.Self.UserTime + ru.Self.SystemTime
	}
	if ru.Children != nil {
		total += ru.Children.UserTime + ru.Children.SystemTime
	}
	return total
}

func (self *runner) Complete() {
	self.done()
	target := core.CompleteFile
	if self.monitoring && self.jobInfo.RusageInfo != nil {
		if t := time.Since(self.start); t > time.Minute*15 {
			if threads := totalCpu(self.jobInfo.RusageInfo) /
				t.Seconds(); threads > 1.5*self.jobInfo.Threads {
				target = core.Errors
				if writeError := self.metadata.WriteRaw(target, fmt.Sprintf(
					"Stage exceeded its threads quota (using %.1f, allowed %g)",
					threads,
					self.jobInfo.Threads)); writeError != nil {
					util.PrintError(writeError, "monitor",
						"Could not write errors file.")
				}
			}
		}
		if target != core.Errors {
			if exceededMemReservation, maxRss := self.exceededMemReservation(); exceededMemReservation {
				target = core.Errors
				if writeError := self.metadata.WriteRaw(target, fmt.Sprintf(
					"%s (using %.1f, allowed %g)",
					core.ExceededMemQuotaMessage,
					float64(maxRss)/(1024*1024*1024),
					self.jobInfo.MemGB)); writeError != nil {
					util.PrintError(writeError, "monitor",
						"Could not write errors file.")
				}
			}
		}
	}
	if target == core.CompleteFile {
		if writeError := self.metadata.WriteTime(core.CompleteFile); writeError != nil {
			util.PrintError(writeError, "monitor", "Could not write complete file.")
		}
	}
	self.sync()
	if jErr := self.metadata.UpdateJournal(target); jErr != nil {
		util.PrintError(jErr, "monitor", "Could not update %v journal file.", target)
	}
	self.waitForPerf()
	os.Exit(0)
}

func (self *runner) sync() {
	if self.runType == "split" {
		syncFile(self.metadata.MetadataFilePath(core.StageDefsFile))
	} else {
		syncFile(self.metadata.MetadataFilePath(core.OutsFile))
	}
	syncFile(path.Dir(self.metadata.FilePath("nil")))
	syncFile(path.Dir(self.metadata.MetadataFilePath(core.CompleteFile)))
}

func (self *runner) StartJob(args []string) error {
	if self.supervisor == nil {
		self.supervisor = newProcessSupervisor()
	}
	cmd := exec.Command(args[0], args[1:]...)
	control, err := newControlChannel(self.log)
	if err != nil {
		return err
	}
	self.control = control
	defer control.closeChild()
	// We really don't want the child outliving the parent.
	self.supervisor.configure(cmd, syscall.SIGKILL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if pc := self.jobInfo.ProfileConfig; pc != nil && len(pc.Env) > 0 {
		cmd.Env = pc.MakeEnv(
			self.metadata.MetadataFilePath(core.PerfData),
			self.metadata.MetadataFilePath(core.ProfileOut))
	}
	control.configure(cmd)
	if err := func(cmd *exec.Cmd) error {
		if self.monitoring && core.ShouldSetVMemRLimit(self.jobInfo) {
			// Exclude mrjob's vmem usage from the rlimit.
			mem, _ := self.resourceSampler().ProcessTreeMemory(self.jobInfo.Pid, true, nil)
			amount := int64(self.jobInfo.VMemGB)*1024*1024*1024 - mem.Vmem
			if amount < mem.Vmem+1024*1024 {
				amount = mem.Vmem + 1024*1024
			}
			if oldAmount, err := core.SetVMemRLimit(uint64(amount)); err != nil {
				util.LogError(err, "monitor",
					"Could not set VM rlimit.")
			} else {
				// After launching the subprocess, restore the vmem
				// limit for this process.  Otherwise the go runtime can run
				//  into various kinds of trouble.
				defer func(amt uint64) {
					if _, err := core.SetVMemRLimit(amt); err != nil {
						util.LogError(err, "monitor",
							"Could not restore VM rlimit.")
					}
				}(oldAmount)
			}
		}
		if err := func() error {
			util.EnterCriticalSection()
			defer util.ExitCriticalSection()
			self.job = cmd
			return self.supervisor.start(cmd)
		}(); err != nil {
			self.control.closeParent()
			return err
		}
		return nil
	}(cmd); err != nil {
		return err
	}
	if err := self.startProfile(); err != nil {
		util.PrintError(err, "monitor", "Could not start profiling.")
	}
	return nil
}

func (self *runner) startProfile() error {
	var cmd *exec.Cmd
	var journaledFiles []core.MetadataFileName
	if perfArgs := os.Getenv("MRO_PERF_ARGS"); perfArgs != "" &&
		self.jobInfo.ProfileMode == core.PerfRecordProfile {
		// For backwards compatibility, ignore the custom config.
		journaledFiles = []core.MetadataFileName{core.PerfData}
		if args, err := shlex.Split(perfArgs); err != nil {
			util.PrintError(err, "profile", "Error parsing perf args")
			return nil
		} else {
			baseArgs := []string{
				"record",
				"-p", strconv.Itoa(self.job.Process.Pid),
				"-o", self.metadata.MetadataFilePath(journaledFiles[0]),
			}
			cmd = exec.Command("perf", append(baseArgs, args...)...)
		}
	} else if pc := self.jobInfo.ProfileConfig; pc == nil || pc.Command == "" {
		return nil
	} else {
		cmd = exec.Command(pc.Command, pc.ExpandedArgs(
			self.metadata.MetadataFilePath(core.PerfData),
			self.metadata.MetadataFilePath(core.ProfileOut),
			self.job.Process.Pid)...)
	}
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if self.supervisor == nil {
		self.supervisor = newProcessSupervisor()
	}
	if err := self.supervisor.startAuxiliary(cmd, syscall.SIGINT); err != nil {
		return err
	} else {
		perfDone := make(chan struct{})
		self.perfDone = perfDone
		go func(cmd *exec.Cmd, c chan<- struct{}) {
			cmd.Wait()
			close(c)
		}(cmd, perfDone)
		for _, file := range journaledFiles {
			self.metadata.UpdateJournal(file)
		}
		return nil
	}
}

func (self *runner) killJobProcess(sig syscall.Signal, reason string) {
	if self.supervisor != nil {
		self.supervisor.kill(sig, reason)
	}
}

func (self *runner) HandleSignal(sig os.Signal) {
	util.PrintInfo("monitor", "Caught signal %v", sig)
	cmd := self.job
	if cmd != nil {
		self.killJobProcess(syscall.SIGKILL, fmt.Sprintf("caught signal %v", sig))
	}
	if c := self.isDone; c != nil {
		t := time.NewTimer(time.Second * 5)
		// Wait up to 5 seconds for the child process to terminate.
		select {
		case <-t.C:
		case <-c:
		}
		t.Stop()
	}
	self.done()
	if err := self.metadata.WriteRaw(core.Errors, fmt.Sprintf("Caught signal %v", sig)); err != nil {
		util.PrintError(err, "monitor", "Could not write errors file.")
	}
	if err := self.metadata.UpdateJournal(core.Errors); err != nil {
		util.PrintError(err, "monitor", "Could not update error journal file.")
	}
}

// Reads at most n bytes from the reader, returning when either n bytes are read
// or the end of the reader is reached.  Errors are ignored.
func readBytes(n int, reader *os.File) []byte {
	if n <= 0 {
		panic("Cannot read non-positive number of bytes!")
	}
	result := make([]byte, n)
	cursor := 0
	for {
		lastRead, err := reader.Read(result[cursor:])
		if lastRead > 0 {
			cursor += lastRead
		}
		if err != nil || lastRead <= 0 || cursor >= n {
			return result[:cursor]
		}
	}
}

// This error contains a string written by the stage code.  It is already
// formatted, so when this is seen, any additional message is ignored.
type stageReturnedError struct {
	message string
}

func (self *stageReturnedError) Error() string {
	return self.message
}

// Wait for the process to complete or, if monitoring is enabled, for it to
// exceed its memory quota.
func (self *runner) WaitLoop() {
	wait := make(chan error, 1)
	go func(wait chan<- error) {
		defer func(wait chan<- error) {
			close(wait)
		}(wait)
		errorBytes := self.control.readError(8100)
		if len(errorBytes) > 0 {
			// If the job has finished, we want to wait on it so it isn't
			// a zombie while we do our cleanup, and also so that its rusage
			// gets reported.  However, if it doesn't die we don't want to
			// block our own exit.  Under most circumstances the process will
			// have already terminated by the time we get here.
			go func() {
				if self.supervisor != nil {
					self.supervisor.wait()
				}
				close(self.isDone)
			}()
			wait <- &stageReturnedError{message: string(errorBytes)}
		} else {
			close(self.isDone)
			var err error
			if self.supervisor == nil {
				err = nil
			} else {
				err = sigToErr(self.supervisor.wait())
			}
			if errorBytes := self.control.readErrorAfterWait(8100); len(errorBytes) > 0 {
				wait <- &stageReturnedError{message: string(errorBytes)}
			} else {
				wait <- err
			}
		}
	}(wait)
	err := func(wait <-chan error) error {
		defer self.control.closeParent()
		// Make sure we record at least one memory high-water mark, even
		// for short stages.
		self.updateChildMemGB()
		lastHeartbeat := time.Now()
		// Do the first memory sample after just 500ms, to capture information
		// about very short stages.
		timer := time.NewTimer(time.Millisecond * 500)
		defer timer.Stop()
		for {
			select {
			case err := <-wait:
				return err
			case <-timer.C:
				// Minimize parent process impact on memory stats, and
				// prevent mrjob from using too many resources for polling.
				runtime.GC()
				if err := self.monitor(&lastHeartbeat); err != nil {
					return err
				}
			}
			// Don't start a new timer going if we're already done.
			select {
			case err := <-wait:
				return err
			default:
			}
			if self.monitoring {
				timer.Reset(MonitorMemorySampleInterval)
			} else {
				timer.Reset(MemorySampleInterval)
			}
		}
	}(wait)
	{
		// Wait up to 5 seconds for the job to finish, to ensure we get rusage.
		select {
		case <-time.After(time.Second * 5):
			self.killJobProcess(syscall.SIGKILL,
				"stage process did not exit within 5 seconds after monitor finished")
		case <-self.isDone:
		}
	}
	util.EnterCriticalSection()
	defer util.ExitCriticalSection()
	if err != nil {
		self.Fail(err, "Job failed in stage code")
	} else {
		self.Complete()
	}
}

func (self *runner) updateChildMemGB() {
	proc := self.job.Process
	if proc == nil {
		return
	}
	io := make(map[int]*core.IoAmount)
	sampler := self.resourceSampler()
	mem, err := sampler.ProcessTreeMemory(proc.Pid, true, io)
	if selfMem, err := sampler.RunningMemory(self.jobInfo.Pid); err == nil {
		// Do this rather than just calling core.GetProcessTreeMemory,
		// above, because we don't want to include the profiling child
		// process (if any).
		mem.Add(selfMem)
	}
	mem.IncreaseRusage(sampler.Rusage())
	self.highMem.IncreaseTo(mem)
	if err != nil {
		util.LogError(err, "monitor",
			"Error updating job statistics. Final statistics may not be accurite.")
	} else {
		self.ioStats.Update(io, time.Now())
	}
}

func (self *runner) logProcessTree() {
	tree, _ := self.resourceSampler().ProcessTreeMemoryList(os.Getpid())
	if len(tree) > 0 {
		util.LogInfo("monitor", "Process tree:\n%s",
			tree.Format("       "))
	}
}

func (self *runner) monitor(lastHeartbeat *time.Time) error {
	self.updateChildMemGB()
	vmemGB := float64(self.highMem.Vmem) / (1024 * 1024 * 1024)
	if exceeded, rssBytes := self.exceededMemReservation(); exceeded {
		rssGB := float64(rssBytes) / (1024 * 1024 * 1024)
		self.logProcessTree()
		self.writeMemViolation(rssBytes)

		if self.monitoring {
			if proc := self.job.Process; proc != nil {
				tree, _ := self.resourceSampler().ProcessTreeMemoryList(proc.Pid)
				if len(tree) > 0 {
					util.LogInfo("monitor", "Process tree:\n%s",
						tree.Format("       "))
				}
			}
			reason := fmt.Sprintf("%s (using %.1f, allowed %gG)",
				core.ExceededMemQuotaMessage,
				rssGB, self.jobInfo.MemGB)
			self.killJobProcess(syscall.SIGKILL, reason)

			return fmt.Errorf("%s", reason)
		} else {
			util.LogInfo("monitor",
				"%s (using %.1f, allowed %gG)",
				core.ExceededMemQuotaMessage,
				rssGB, self.jobInfo.MemGB)
		}
	} else if core.ShouldCheckVMem(self.jobInfo) && vmemGB > self.jobInfo.VMemGB {
		self.logProcessTree()
		if self.monitoring {
			reason := core.VMemQuotaMessage(vmemGB, self.jobInfo.VMemGB)
			self.killJobProcess(syscall.SIGKILL, reason)
			return fmt.Errorf("%s", reason)
		} else {
			util.LogInfo("monitor", "%s", core.VMemQuotaMessage(vmemGB, self.jobInfo.VMemGB))
		}
	}
	if time.Since(*lastHeartbeat) > HeartbeatInterval {
		if err := self.metadata.UpdateJournal(core.Heartbeat); err != nil {
			util.PrintError(err, "monitor", "Could not write heartbeat.")
		} else {
			*lastHeartbeat = time.Now()
		}
		if _, err := os.Stat(self.metadata.MetadataFilePath(core.LogFile)); os.IsNotExist(err) {
			reason := "Stage log file has been deleted.  Aborting run.\n" +
				"  This is usually the result of `mrp` thinking the stage failed\n" +
				"  and deleting the stage directory in order to retry."
			self.killJobProcess(syscall.SIGKILL, reason)
			return fmt.Errorf("%s", reason)
		}
	}
	return nil
}

// Returns true if we exceeded the memory reservation.
// Returns the max observed RSS in bytes in second position.
func (self *runner) exceededMemReservation() (bool, int64) {
	maxRss := self.highMem.Rss
	if self.jobInfo == nil {
		return false, maxRss
	}
	if self.jobInfo.RusageInfo != nil && self.jobInfo.RusageInfo.Children != nil {
		maxRusage := int64(self.jobInfo.RusageInfo.Children.MaxRss) * 1024
		if maxRusage > maxRss {
			maxRss = maxRusage
		}
	}
	// jobmanager rounds up to the nearest MB
	res := int64(math.Ceil(self.jobInfo.MemGB*1024) * 1024 * 1024)
	return (maxRss > res), maxRss
}

func (self *runner) writeMemViolation(maxRss int64) {
	var memResGB float64
	if self.jobInfo != nil {
		memResGB = self.jobInfo.MemGB
	}
	contents := core.MemViolationContents{
		MemReservationGB: memResGB,
		MaxRssBytes:      maxRss,
	}
	if err := self.metadata.WriteAtomic(core.MemViolation, contents); err != nil {
		util.LogError(err, "monitor", "Unable to write %s file.", core.MemViolation)
		return
	}
	_ = self.metadata.UpdateJournal(core.MemViolation)
}
