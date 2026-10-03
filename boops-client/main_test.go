package main

import (
	"boops/client"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
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
