package core

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"runtime/trace"
	"strings"
	"testing"
	"time"

	"github.com/martian-lang/martian/martian/util"
)

func TestPipestancePythonAdapterSmoke(t *testing.T) {
	mrjobPath := os.Getenv("MRO_TEST_MRJOB")
	if mrjobPath == "" {
		t.Skip("MRO_TEST_MRJOB is required for the mrjob-backed adapter smoke test")
	}
	if _, err := os.Stat(mrjobPath); err != nil {
		t.Fatalf("MRO_TEST_MRJOB=%s is not usable: %v", mrjobPath, err)
	}
	adaptersPath, err := filepath.Abs("../../adapters")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(adaptersPath, "python", "martian_shell.py")); err != nil {
		t.Fatalf("adapter path %s is not usable: %v", adaptersPath, err)
	}

	const src = `
stage PYTHON_ADAPTER_SMOKE(
    in  string message,
    out file   result,
    src py     "python_adapter_smoke",
)

pipeline PYTHON_ADAPTER_PIPELINE(
    in  string message,
    out file   result,
)
{
    call PYTHON_ADAPTER_SMOKE(
        message = self.message,
    )

    return (
        result = PYTHON_ADAPTER_SMOKE.result,
    )
}

call PYTHON_ADAPTER_PIPELINE(
    message = "hello from adapter",
)
`

	utilLogger := testLogger{t: t}
	util.SetPrintLogger(&utilLogger)
	defer util.SetPrintLogger(&devNull)

	rtOpts := DefaultRuntimeOptions()
	rtOpts.Debug = true
	rt := Runtime{
		Config:       &rtOpts,
		mrjob:        mrjobPath,
		adaptersPath: adaptersPath,
	}
	rt.jobConfig = &JobManagerJson{
		JobSettings: &JobManagerSettings{
			ThreadsPerJob: 1,
			MemGBPerJob:   1,
			ExtraVmemGB:   1,
			ThreadEnvs:    []string{"GOMAXPROCS"},
		},
	}
	rt.LocalJobManager, err = NewLocalJobManager(2, 2, 0, true, false, false, rt.jobConfig)
	if err != nil {
		t.Fatal(err)
	}
	rt.JobManager = rt.LocalJobManager
	psdir := t.TempDir()

	pipestance, err := rt.InvokePipeline(src,
		"testdata/python_adapter_smoke.mro", t.Name(),
		psdir, []string{"testdata"}, "<none>", map[string]string{}, nil)
	if err != nil {
		t.Fatal("invoking pipeline:", err)
	}
	defer pipestance.Unlock()
	pipestance.LoadMetadata(context.Background())
	runPipestanceToCompletion(t, pipestance, rt.LocalJobManager.Done())

	stageNodePath := path.Join(psdir, "PYTHON_ADAPTER_PIPELINE", "PYTHON_ADAPTER_SMOKE")
	outsPath := path.Join(stageNodePath, defaultFork, OutsFile.FileName())
	outsData, err := os.ReadFile(outsPath)
	if err != nil {
		t.Fatal(err)
	}
	var outs struct {
		Result string `json:"result"`
	}
	if err := json.Unmarshal(outsData, &outs); err != nil {
		t.Fatal(err)
	}
	if outs.Result == "" {
		t.Fatalf("missing result in %s: %s", outsPath, string(outsData))
	}
	resultData, err := os.ReadFile(outs.Result)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(string(resultData)); got != "hello from adapter" {
		t.Fatalf("result = %q, want %q", got, "hello from adapter")
	}

	stagePath := path.Join(stageNodePath, defaultFork, "chnk0")
	if logData, err := os.ReadFile(path.Join(stagePath, LogFile.FileName())); err != nil {
		t.Fatal(err)
	} else if !bytes.Contains(logData, []byte("python adapter smoke log")) {
		t.Fatalf("expected adapter log in _log, got %s", string(logData))
	}
	if progressData, err := os.ReadFile(path.Join(stagePath, ProgressFile.FileName())); err != nil {
		t.Fatal(err)
	} else if !bytes.Contains(progressData, []byte("python adapter smoke progress")) {
		t.Fatalf("expected adapter progress, got %s", string(progressData))
	}
	var jobInfo JobInfo
	if err := NewMetadata("", stagePath).ReadInto(JobInfoFile, &jobInfo); err != nil {
		t.Fatal(err)
	}
	if jobInfo.PythonInfo == nil {
		t.Fatalf("expected Python adapter jobinfo, got %+v", jobInfo)
	}
}

func runPipestanceToCompletion(t *testing.T, pipestance *Pipestance, localJobDone <-chan struct{}) {
	t.Helper()
	ctx, task := trace.NewTask(context.Background(), "python-adapter-smoke")
	defer task.End()
	ti := time.NewTimer(0)
	if !ti.Stop() {
		<-ti.C
	}
	deadline := time.After(30 * time.Second)
	for {
		flushChannel(localJobDone)
		done, hadProgress := loopBody(t, pipestance)
		if done {
			return
		}
		if !hadProgress {
			ti.Reset(250 * time.Millisecond)
			select {
			case <-ti.C:
			case <-localJobDone:
				if !ti.Stop() {
					<-ti.C
				}
			case <-deadline:
				t.Fatal("timed out waiting for pipestance completion")
			}
		}
		pipestance.CheckHeartbeats(ctx)
	}
}
