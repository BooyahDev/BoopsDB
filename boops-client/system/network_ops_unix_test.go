//go:build unix

package system

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
)

func TestHostWriteFilePreservesModeUnderUmask(t *testing.T) {
	if os.Getenv("BOOPS_TEST_FILE_MODE_CHILD") == "1" {
		old := syscall.Umask(0022)
		defer syscall.Umask(old)
		path := os.Getenv("BOOPS_TEST_FILE_MODE_PATH")
		if err := RealOps().WriteFile(path, []byte("fixture only\n"), 0660); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0660 {
			t.Fatalf("requested mode 0660 became %04o", info.Mode().Perm())
		}
		o := newFixtureOps(t, "netplan")
		o.put(netplanPath, netplanOriginal, 0660)
		failed := false
		o.fail = func(name string, args []string) bool {
			if name == "netplan" && args[0] == "apply" && !failed {
				failed = true
				return true
			}
			return false
		}
		if err := ApplyNetworkSettingsWithOps(twoNICs(), o); err == nil {
			t.Fatal("fixture apply failure ignored")
		}
		data, err := o.ReadFile(netplanPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != netplanOriginal {
			t.Fatal("original bytes not restored")
		}
		info, err = os.Stat(o.local(netplanPath))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0660 {
			t.Fatalf("restored mode 0660 became %04o", info.Mode().Perm())
		}
		return
	}
	path := filepath.Join(t.TempDir(), "mode-fixture")
	cmd := exec.Command(os.Args[0], "-test.run=^TestHostWriteFilePreservesModeUnderUmask$")
	cmd.Env = append(os.Environ(), "BOOPS_TEST_FILE_MODE_CHILD=1", "BOOPS_TEST_FILE_MODE_PATH="+path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("isolated mode test failed: %v\n%s", err, out)
	}
}
