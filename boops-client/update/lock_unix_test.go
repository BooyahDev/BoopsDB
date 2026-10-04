//go:build linux || darwin

package update

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestLockSurvivesExistingFileAndReleases(t *testing.T) {
	p := filepath.Join(t.TempDir(), "update.lock")
	if err := os.WriteFile(p, []byte("stale file"), 0600); err != nil {
		t.Fatal(err)
	}
	a, err := acquireLock(p)
	if err != nil {
		t.Fatal(err)
	}
	if b, err := acquireLock(p); err == nil {
		b.Close()
		t.Fatal("concurrent lock accepted")
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := acquireLock(p)
	if err != nil {
		t.Fatal(err)
	}
	b.Close()
}

func TestLockIsProcessShared(t *testing.T) {
	if path := os.Getenv("BOOPS_LOCK_TEST_PATH"); path != "" {
		lock, err := acquireLock(path)
		if os.Getenv("BOOPS_LOCK_EXPECT_BUSY") == "1" {
			if !errors.Is(err, errLocked) {
				t.Fatalf("expected busy lock, got %v", err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		lock.Close()
		return
	}
	path := filepath.Join(t.TempDir(), "update.lock")
	lock, err := acquireLock(path)
	if err != nil {
		t.Fatal(err)
	}
	runChild := func(expectBusy bool) {
		command := exec.Command(os.Args[0], "-test.run=^TestLockIsProcessShared$")
		command.Env = append(os.Environ(), "BOOPS_LOCK_TEST_PATH="+path)
		if expectBusy {
			command.Env = append(command.Env, "BOOPS_LOCK_EXPECT_BUSY=1")
		}
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("child lock failed: %v %s", err, output)
		}
	}
	runChild(true)
	if err := lock.Close(); err != nil {
		t.Fatal(err)
	}
	runChild(false)
}
