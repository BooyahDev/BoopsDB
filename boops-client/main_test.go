package main

import (
	"boops/client"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"testing"
)

func testMachine() client.Machine {
	return client.Machine{ID: "70ae9891-fc07-45b9-8364-3ab159ee2048", Hostname: "host", Interfaces: []client.InterfaceInfo{{Name: "eth0", IPs: []client.IPInfo{{IP: "192.0.2.10", Subnet: "255.255.255.0"}}}}}
}

func TestSyncApplyFailureDoesNotSaveState(t *testing.T) {
	saves := 0
	failure := errors.New("apply failed")
	err := syncNetworkState(testMachine(), nil, func([]client.InterfaceInfo) error { return failure }, func(*client.MachineState) error { saves++; return nil })
	if !errors.Is(err, failure) {
		t.Fatalf("got %v, want apply failure", err)
	}
	if saves != 0 {
		t.Fatal("state saved after failed network apply")
	}
}

func TestSyncSuccessSavesNormalizedStateAndSkipsUnchangedNetwork(t *testing.T) {
	machine := testMachine()
	machine.Interfaces[0].Gateway = " 0.0.0.0 "
	var state *client.MachineState
	applies := 0
	apply := func(interfaces []client.InterfaceInfo) error {
		applies++
		if interfaces[0].Gateway != "" {
			t.Fatal("gateway not normalized")
		}
		return nil
	}
	save := func(s *client.MachineState) error { state = s; return nil }
	if err := syncNetworkState(machine, nil, apply, save); err != nil {
		t.Fatal(err)
	}
	if state == nil || state.Interfaces[0].Gateway != "" || state.Hostname != "host" {
		t.Fatalf("bad saved state: %+v", state)
	}
	if err := syncNetworkState(machine, state, apply, save); err != nil {
		t.Fatal(err)
	}
	if applies != 1 {
		t.Fatalf("unchanged settings applied %d times", applies)
	}
}

func TestSyncInvalidNetworkDoesNotApplyOrSave(t *testing.T) {
	machine := testMachine()
	machine.Interfaces = append(machine.Interfaces, machine.Interfaces[0])
	calls := 0
	if err := syncNetworkState(machine, nil, func([]client.InterfaceInfo) error { calls++; return nil }, func(*client.MachineState) error { calls++; return nil }); err == nil {
		t.Fatal("duplicate NIC accepted")
	}
	if calls != 0 {
		t.Fatal("invalid network caused a side effect")
	}
}

func TestRegisterBindsExistingMachineWithoutRemoteWrites(t *testing.T) {
	machine := testMachine()
	var requests []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests = append(requests, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"70ae9891-fc07-45b9-8364-3ab159ee2048","interfaces":[]}`))
	}))
	defer server.Close()
	original := apiBase
	apiBase = server.URL + "/api/machines"
	t.Cleanup(func() { apiBase = original })
	saved := ""
	if err := registerMachine(machine.ID, fetchMachine, func(id string) error { saved = id; return nil }); err != nil {
		t.Fatal(err)
	}
	if saved != machine.ID {
		t.Fatal("existing machine not bound")
	}
	if strings.Join(requests, ",") != "GET /api/machines/"+machine.ID {
		t.Fatalf("registration changed remote settings: %v", requests)
	}
}

func TestRegisterFailureDoesNotSave(t *testing.T) {
	for _, tc := range []struct {
		name    string
		machine client.Machine
		err     error
	}{
		{"not found", client.Machine{}, errors.New("404")}, {"ID mismatch", client.Machine{ID: "another-id"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saves := 0
			err := registerMachine(testMachine().ID, func(string) (client.Machine, error) { return tc.machine, tc.err }, func(string) error { saves++; return nil })
			if err == nil || saves != 0 {
				t.Fatalf("err=%v saves=%d", err, saves)
			}
		})
	}
}

func TestFetchMachineRejectsHTTPErrorAndInvalidJSON(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status int
		body   string
	}{{"404", 404, `{"id":"missing"}`}, {"invalid JSON", 200, "{"}} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
				_, _ = w.Write([]byte(tc.body))
			}))
			defer server.Close()
			original := apiBase
			apiBase = server.URL
			defer func() { apiBase = original }()
			if _, err := fetchMachine("test-id"); err == nil {
				t.Fatal("invalid response accepted")
			}
		})
	}
}

func TestSyncChecksUpdateBeforeAPIAndStopsAfterReplacement(t *testing.T) {
	for _, tc := range []struct {
		name      string
		updated   bool
		updateErr error
		wantSync  bool
	}{
		{"replacement", true, nil, false}, {"replacement with reporting error", true, errors.New("state write failed"), false}, {"update failed", false, errors.New("offline"), true}, {"no update", false, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := []string{}
			cfg := &client.Config{ID: "id"}
			err := syncWithUpdate(cfg, func(bool) (bool, error) { calls = append(calls, "update"); return tc.updated, tc.updateErr }, func(string) error { calls = append(calls, "sync"); return nil })
			if err != nil {
				t.Fatal(err)
			}
			want := "update"
			if tc.wantSync {
				want += ",sync"
			}
			if strings.Join(calls, ",") != want {
				t.Fatalf("calls=%v want=%s", calls, want)
			}
		})
	}
}

func TestSyncWithAutoUpdateDisabledSkipsCheck(t *testing.T) {
	disabled := false
	cfg := &client.Config{ID: "machine", AutoUpdate: &disabled}
	err := syncWithUpdate(cfg, func(bool) (bool, error) {
		t.Fatal("disabled auto update was checked")
		return false, nil
	}, func(id string) error {
		if id != "machine" {
			t.Fatalf("wrong machine ID: %s", id)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInventoryJSONEscapesValuesAndReportsHTTPFailure(t *testing.T) {
	var payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.Header.Get("Content-Type") != "application/json" {
			t.Fatal("invalid inventory request")
		}
		data, _ := io.ReadAll(r.Body)
		payload = string(data)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()
	err := putMachineField(server.URL, map[string]string{"cpu_info": "model \"quoted\"\nline"})
	if err == nil {
		t.Fatal("HTTP failure accepted")
	}
	var decoded map[string]string
	if err := json.Unmarshal([]byte(payload), &decoded); err != nil {
		t.Fatalf("invalid JSON payload: %s", payload)
	}
	if decoded["cpu_info"] != "model \"quoted\"\nline" {
		t.Fatal("inventory value changed")
	}
}

func TestHostnameFailureContinuesNetworkWithoutSavingDesiredHostname(t *testing.T) {
	for _, prior := range []string{"", "prior-host"} {
		t.Run(prior, func(t *testing.T) {
			machine := testMachine()
			var previous *client.MachineState
			if prior != "" {
				previous = &client.MachineState{Hostname: prior}
			}
			applies := 0
			var saved *client.MachineState
			err := syncMachineSettings(machine, previous, "linux", func(string, ...string) ([]byte, error) { return nil, exec.ErrNotFound }, func([]client.InterfaceInfo) error { applies++; return nil }, func(s *client.MachineState) error { saved = s; return nil })
			if applies != 1 {
				t.Fatalf("network apply calls=%d, want 1 after missing hostnamectl; error=%v", applies, err)
			}
			if err != nil {
				t.Fatal(err)
			}
			if saved == nil || saved.Hostname != prior {
				t.Fatalf("failed hostname recorded as applied: %+v", saved)
			}
			retries := 0
			if err := syncMachineSettings(machine, saved, "linux", func(string, ...string) ([]byte, error) { retries++; return nil, nil }, func([]client.InterfaceInfo) error { t.Fatal("unchanged network reapplied"); return nil }, func(s *client.MachineState) error { saved = s; return nil }); err != nil {
				t.Fatal(err)
			}
			if retries != 1 || saved.Hostname != "host" {
				t.Fatalf("hostname was not retried: retries=%d state=%+v", retries, saved)
			}
		})
	}
}

func TestWindowsHostnameDoesNotRunLinuxCommandOrBlockNetwork(t *testing.T) {
	applies := 0
	var saved *client.MachineState
	err := syncMachineSettings(testMachine(), nil, "windows", func(string, ...string) ([]byte, error) {
		t.Fatal("Linux hostnamectl executed on Windows")
		return nil, nil
	}, func([]client.InterfaceInfo) error { applies++; return nil }, func(s *client.MachineState) error { saved = s; return nil })
	if err != nil || applies != 1 || saved == nil || saved.Hostname != "" {
		t.Fatalf("err=%v applies=%d state=%+v", err, applies, saved)
	}
}

func TestHostnameCommandUsesSeparateArgumentsAfterFullValidation(t *testing.T) {
	machine := testMachine()
	machine.Hostname = "host; touch /tmp/should-not-exist"
	calls := 0
	run := func(name string, args ...string) ([]byte, error) {
		calls++
		if name != "hostnamectl" || strings.Join(args, "|") != "set-hostname|--|host; touch /tmp/should-not-exist" {
			t.Fatalf("hostname command incorrectly formed: %s %v", name, args)
		}
		return nil, nil
	}
	if err := syncMachineSettings(machine, nil, "linux", run, func([]client.InterfaceInfo) error { return nil }, func(*client.MachineState) error { return nil }); err != nil {
		t.Fatal(err)
	}
	machine.Interfaces = append(machine.Interfaces, machine.Interfaces[0])
	if err := syncMachineSettings(machine, nil, "linux", run, func([]client.InterfaceInfo) error { t.Fatal("invalid network applied"); return nil }, func(*client.MachineState) error { t.Fatal("invalid network saved"); return nil }); err == nil {
		t.Fatal("duplicate NIC accepted")
	}
	if calls != 1 {
		t.Fatal("hostname modified before validation")
	}
}

func TestRegisterUppercaseUUIDAcceptsCanonicalAPIResponse(t *testing.T) {
	upper := strings.ToUpper(testMachine().ID)
	machine := testMachine()
	saved := ""
	err := registerMachine(upper, func(string) (client.Machine, error) { return machine, nil }, func(id string) error { saved = id; return nil })
	if err != nil {
		t.Fatalf("valid uppercase UUID rejected: %v", err)
	}
	if saved != upper {
		t.Fatalf("registration ID rewritten: %s", saved)
	}
}

func TestSyncUppercaseStoredUUIDAcceptsCanonicalAPIResponse(t *testing.T) {
	upper := strings.ToUpper(testMachine().ID)
	cfg := &client.Config{ID: upper}
	machine := testMachine()
	machine.Hostname = ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Error("sync verification performed remote write")
			w.WriteHeader(405)
			return
		}
		_ = json.NewEncoder(w).Encode(machine)
	}))
	defer server.Close()
	original := apiBase
	apiBase = server.URL
	defer func() { apiBase = original }()
	applies := 0
	err := syncWithUpdate(cfg, func(bool) (bool, error) { return false, nil }, func(id string) error {
		return syncRegisteredMachine(id, fetchMachine, func(m client.Machine) error {
			return syncMachineSettings(m, nil, "windows", func(string, ...string) ([]byte, error) { t.Fatal("unexpected hostname command"); return nil, nil }, func([]client.InterfaceInfo) error { applies++; return nil }, func(*client.MachineState) error { return nil })
		})
	})
	if err != nil {
		t.Fatalf("valid stored uppercase UUID rejected: %v", err)
	}
	if applies != 1 {
		t.Fatalf("network synchronization was not reached: %d", applies)
	}
	if cfg.ID != upper {
		t.Fatalf("existing config ID rewritten: %s", cfg.ID)
	}
}

func TestMalformedUUIDRejectedBeforeFetchAndLocalWrites(t *testing.T) {
	for _, id := range []string{"", "another-id", "70ae9891fc0745b983643ab159ee2048", "70ae9891-fc07-45b9-8364-3ab159ee204g", " 70ae9891-fc07-45b9-8364-3ab159ee2048", "70ae9891-fc07-45b9-8364-3ab159ee2048/extra"} {
		t.Run(id, func(t *testing.T) {
			fetches, writes := 0, 0
			fetch := func(string) (client.Machine, error) { fetches++; return client.Machine{ID: id}, nil }
			if err := registerMachine(id, fetch, func(string) error { writes++; return nil }); err == nil {
				t.Error("malformed registration ID accepted")
			}
			if err := syncRegisteredMachine(id, fetch, func(client.Machine) error { writes++; return nil }); err == nil {
				t.Error("malformed stored ID accepted")
			}
			if fetches != 0 || writes != 0 {
				t.Fatalf("malformed UUID caused side effects: fetches=%d writes=%d", fetches, writes)
			}
		})
	}
}

func TestDifferentOrMalformedAPIUUIDCannotBindOrSync(t *testing.T) {
	for _, responseID := range []string{"70ae9891-fc07-45b9-8364-3ab159ee2049", "not-a-uuid", ""} {
		t.Run(responseID, func(t *testing.T) {
			calls := 0
			fetch := func(string) (client.Machine, error) { return client.Machine{ID: responseID}, nil }
			if err := registerMachine(testMachine().ID, fetch, func(string) error { calls++; return nil }); err == nil {
				t.Error("different or malformed API ID bound")
			}
			if err := syncRegisteredMachine(testMachine().ID, fetch, func(client.Machine) error { calls++; return nil }); err == nil {
				t.Error("different or malformed API ID synchronized")
			}
			if calls != 0 {
				t.Fatal("API ID mismatch caused a local write")
			}
		})
	}
}
