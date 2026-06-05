//go:build darwin && cgo
// +build darwin,cgo

package core

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/martian-lang/martian/martian/util"
)

func testPython(t *testing.T) string {
	t.Helper()
	if python, err := exec.LookPath("python"); err == nil {
		return python
	}
	if python, err := exec.LookPath("python3"); err == nil {
		return python
	}
	t.Fatal("expected python or python3 in PATH")
	return ""
}

func runGetProcessVsize(t *testing.T, vsize, rss string) (ObservedMemory, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, testPython(t), "testdata/vsize.py", vsize, rss)
	cmd.SysProcAttr = util.Pdeathsig(&syscall.SysProcAttr{}, syscall.SIGKILL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := io.ReadAll(stdout); err != nil {
		t.Error(err)
	}
	mem, err := GetRunningMemory(cmd.Process.Pid)
	if err := stdin.Close(); err != nil {
		t.Error(err)
	}
	return mem, err
}

func runGetProcessTreeMemory(t *testing.T,
	parentVsize, parentRss, childVsize, childRss string) (ObservedMemory, ObservedMemory, ProcessTree, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, testPython(t), "testdata/vsize_tree.py",
		parentVsize, parentRss, childVsize, childRss)
	cmd.SysProcAttr = util.Pdeathsig(&syscall.SysProcAttr{}, syscall.SIGKILL)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cmd.Wait(); err != nil {
			t.Error(err)
		}
	}()
	if _, err := io.ReadAll(stdout); err != nil {
		t.Error(err)
	}
	parentMem, err := GetRunningMemory(cmd.Process.Pid)
	if err != nil {
		_ = stdin.Close()
		return ObservedMemory{}, ObservedMemory{}, nil, err
	}
	treeMem, err := GetProcessTreeMemory(cmd.Process.Pid, true, nil)
	if err != nil {
		_ = stdin.Close()
		return ObservedMemory{}, ObservedMemory{}, nil, err
	}
	tree, err := GetProcessTreeMemoryList(cmd.Process.Pid)
	if err := stdin.Close(); err != nil {
		t.Error(err)
	}
	return parentMem, treeMem, tree, err
}

func TestGetProcessVsize(t *testing.T) {
	t.Parallel()
	if mem, err := runGetProcessVsize(t, "1048576", "10240"); err != nil {
		t.Error(err)
	} else {
		if rss := mem.RssKb(); rss < 10240 {
			t.Errorf("Expected at least 10240kb rss usage, got %d", rss)
		} else if rss > 10*10240 {
			t.Errorf("Expected 10240kb rss usage, plus overhead, got %d", rss)
		}
		if vmem := mem.VmemKb(); vmem <= 0 {
			t.Errorf("Expected positive vmem usage, got %d", vmem)
		} else if vmem > 4*1048576 {
			t.Errorf("Expected Darwin sparse mapping vmem to stay below 4GiB, got %d", vmem)
		}
	}
}

func TestDarwinProcessTreeMemoryIncludesChildren(t *testing.T) {
	t.Parallel()
	parentMem, treeMem, tree, err := runGetProcessTreeMemory(t, "262144", "4096", "262144", "8192")
	if err != nil {
		t.Fatal(err)
	}
	if treeMem.Rss <= parentMem.Rss {
		t.Fatalf("expected tree rss %d to be greater than parent rss %d", treeMem.Rss, parentMem.Rss)
	}
	if treeMem.Vmem <= parentMem.Vmem {
		t.Fatalf("expected tree vmem %d to be greater than parent vmem %d", treeMem.Vmem, parentMem.Vmem)
	}
	if len(tree) < 2 {
		t.Fatalf("expected at least 2 processes in tree, got %d", len(tree))
	}
	foundChild := false
	for _, proc := range tree {
		if proc.Depth > 0 {
			foundChild = true
			if proc.Memory.Rss <= 0 {
				t.Fatalf("expected child rss to be non-zero for pid %d", proc.Pid)
			}
		}
	}
	if !foundChild {
		t.Fatal("expected child process in process tree list")
	}
}
