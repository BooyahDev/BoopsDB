package system

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Ops separates host operations from validation and generation.
type Ops interface {
	OS() string
	Run(name string, args ...string) ([]byte, error)
	ReadFile(path string) ([]byte, error)
	WriteFile(path string, data []byte, mode fs.FileMode) error
	Rename(oldPath, newPath string) error
	Remove(path string) error
	Glob(pattern string) ([]string, error)
	Exists(path string) (bool, error)
	LookPath(name string) (string, error)
}

type hostOps struct{}

func RealOps() Ops         { return hostOps{} }
func (hostOps) OS() string { return runtime.GOOS }
func (hostOps) Run(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput()
}
func (hostOps) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (hostOps) WriteFile(path string, data []byte, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if err := file.Chmod(mode); err != nil {
		return errors.Join(err, file.Close())
	}
	_, err = file.Write(data)
	return errors.Join(err, file.Close())
}
func (hostOps) Rename(a, b string) error              { return os.Rename(a, b) }
func (hostOps) Remove(path string) error              { return os.Remove(path) }
func (hostOps) Glob(pattern string) ([]string, error) { return filepath.Glob(pattern) }
func (hostOps) LookPath(name string) (string, error)  { return exec.LookPath(name) }
func (hostOps) Exists(path string) (bool, error) {
	_, err := os.Stat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

func runChecked(ops Ops, name string, args ...string) error {
	out, err := ops.Run(name, args...)
	if err != nil {
		return fmt.Errorf("%s %v: %w (output: %s)", name, args, err, strings.TrimSpace(string(out)))
	}
	return nil
}

type fileSnapshot struct {
	path   string
	data   []byte
	mode   fs.FileMode
	exists bool
}

func readSnapshot(ops Ops, path string, defaultMode fs.FileMode) (fileSnapshot, error) {
	s := fileSnapshot{path: path, mode: defaultMode}
	var err error
	s.exists, err = ops.Exists(path)
	if err != nil {
		return s, err
	}
	if !s.exists {
		return s, nil
	}
	s.data, err = ops.ReadFile(path)
	if err != nil {
		return s, err
	}
	mode, err := ops.Run("stat", "-c", "%a", path)
	if err != nil {
		return s, fmt.Errorf("read permissions for %s: %w", path, err)
	}
	bits, err := strconv.ParseUint(strings.TrimSpace(string(mode)), 8, 12)
	if err != nil {
		return s, fmt.Errorf("invalid permissions for %s: %w", path, err)
	}
	s.mode = fs.FileMode(bits & 0777)
	return s, nil
}

// replaceFile keeps the temporary file beside its destination for atomic rename.
func replaceFile(ops Ops, path string, data []byte, mode fs.FileMode) error {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		return err
	}
	tmp := fmt.Sprintf("%s.boops-%x.tmp", path, suffix)
	if err := ops.WriteFile(tmp, data, mode); err != nil {
		_ = ops.Remove(tmp)
		return fmt.Errorf("write %s: %w", path, err)
	}
	if err := ops.Rename(tmp, path); err != nil {
		_ = ops.Remove(tmp)
		return fmt.Errorf("replace %s: %w", path, err)
	}
	return nil
}

func applyFile(ops Ops, s fileSnapshot, data []byte, apply func() error) error {
	if err := replaceFile(ops, s.path, data, s.mode); err != nil {
		return err
	}
	if err := apply(); err != nil {
		var restoreErr error
		if s.exists {
			restoreErr = replaceFile(ops, s.path, s.data, s.mode)
		} else {
			restoreErr = ops.Remove(s.path)
		}
		if restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore %s: %w", s.path, restoreErr))
		}
		if restoreErr = apply(); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("reapply restored %s: %w", s.path, restoreErr))
		}
		return err
	}
	return nil
}
