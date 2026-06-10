//go:build windows
// +build windows

package adapter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWindowsAdapterControlFiles(t *testing.T) {
	tmp := t.TempDir()
	logPath := filepath.Join(tmp, "stage.log")
	errorPath := filepath.Join(tmp, "stage.errors")
	t.Setenv(controlLogPathEnv, logPath)
	t.Setenv(controlErrorPathEnv, errorPath)

	logFile := openControlLog()
	if logFile == nil {
		t.Fatal("expected control log file")
	}
	if _, err := logFile.WriteString("adapter log\n"); err != nil {
		t.Fatal(err)
	}
	if err := logFile.Close(); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(errorPath, []byte("old error\n"), 0644); err != nil {
		t.Fatal(err)
	}
	errorFile := openControlErrors()
	if errorFile == nil {
		t.Fatal("expected control error file")
	}
	if _, err := errorFile.WriteString("adapter error\n"); err != nil {
		t.Fatal(err)
	}
	if err := errorFile.Close(); err != nil {
		t.Fatal(err)
	}

	if contents, err := os.ReadFile(logPath); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(contents), "adapter log") {
		t.Fatalf("expected adapter log in %s, got %q", logPath, string(contents))
	}
	if contents, err := os.ReadFile(errorPath); err != nil {
		t.Fatal(err)
	} else if !strings.Contains(string(contents), "adapter error") {
		t.Fatalf("expected adapter error in %s, got %q", errorPath, string(contents))
	} else if strings.Contains(string(contents), "old error") {
		t.Fatalf("expected %s to be truncated, got %q", errorPath, string(contents))
	}
}
