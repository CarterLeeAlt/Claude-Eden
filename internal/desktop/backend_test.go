package desktop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sylvinus/agent-vm/internal/appupdate"
	"github.com/sylvinus/agent-vm/internal/desktopupdate"
)

func TestMain(m *testing.M) {
	if os.Getenv("EDEN_DESKTOP_DRIVER_FIXTURE") == "1" && len(os.Args) > 1 {
		switch os.Args[1] {
		case "app-update":
			fmt.Println(`{"phase":"checking","currentVersion":"1.0.0","message":"fixture"}`)
			if os.Getenv("EDEN_DESKTOP_DRIVER_MODE") == "block" {
				time.Sleep(time.Minute)
			}
			if len(os.Args) > 2 && os.Args[2] == "download" {
				fmt.Println(`{"phase":"downloaded","latestVersion":"2.0.0","archivePath":"fixture.exe","sha256":"fixture"}`)
			} else {
				fmt.Println(`{"phase":"current","currentVersion":"1.0.0"}`)
			}
			os.Exit(0)
		case "prepare-update":
			mode := os.Getenv("EDEN_DESKTOP_DRIVER_MODE")
			if mode == "stop_required" {
				fmt.Print(`{"status":"stop_required"}`)
				os.Exit(2)
			}
			if mode == "failure" {
				fmt.Print(`{"status":"error","message":"fixture failure"}`)
				os.Exit(7)
			}
			fmt.Print(`{"status":"prepared"}`)
			os.Exit(0)
		case "upload-files":
			if os.Getenv("EDEN_DESKTOP_DRIVER_MODE") == "block" {
				time.Sleep(time.Minute)
			}
			time.Sleep(200 * time.Millisecond)
			if os.Getenv("EDEN_DESKTOP_DRIVER_MODE") == "failure" {
				fmt.Print(`{"message":"fixture failure"}`)
				os.Exit(7)
			}
			fmt.Print(`{"ok":true}`)
			os.Exit(0)
		case "clipboard-files":
			if os.Getenv("EDEN_DESKTOP_DRIVER_MODE") == "failure" {
				fmt.Print(`{"message":"fixture cache failure"}`)
				os.Exit(7)
			}
			if len(os.Args) > 5 && os.Args[5] == "receive" {
				_ = json.NewEncoder(os.Stdout).Encode(map[string]any{"cache_id": "fixture-cache", "paths": []string{filepath.Join(filepath.Dir(os.Getenv("EDEN_CONFIG")), "cache", "one")}})
			} else {
				fmt.Print(`{}`)
			}
			os.Exit(0)
		}
	}
	os.Exit(m.Run())
}

func TestBackendProcessFixture(t *testing.T) {
	if os.Getenv("EDEN_DESKTOP_FIXTURE") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "slow":
		time.Sleep(200 * time.Millisecond)
		fmt.Print(`{"status":"ready"}`)
	case "cancel":
		time.Sleep(time.Minute)
	case "failure":
		fmt.Print(`{"status":"error","message":"fixture"}`)
		os.Exit(7)
	case "overflow":
		fmt.Print(strings.Repeat("x", 1<<20))
	case "invalid":
		fmt.Print("not json")
	}
	os.Exit(0)
}

func TestUpdateBackendAdapter(t *testing.T) {
	t.Setenv("EDEN_DESKTOP_DRIVER_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	backend, err := NewBackend(executable, filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	backend.Directory = root
	for _, checkOnly := range []bool{false, true} {
		var states []appupdate.State
		if err := backend.Update(context.Background(), checkOnly, func(state appupdate.State) { states = append(states, state) }); err != nil {
			t.Fatal(err)
		}
		want := "downloaded"
		if checkOnly {
			want = "current"
		}
		if len(states) != 2 || states[0].Phase != "checking" || states[1].Phase != want {
			t.Fatalf("update stream changed: %+v", states)
		}
	}
	for _, stop := range []bool{false, true} {
		if err := backend.PrepareUpdate(context.Background(), stop); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("EDEN_DESKTOP_DRIVER_MODE", "stop_required")
	if err := backend.PrepareUpdate(context.Background(), false); err != desktopupdate.ErrStopRequired {
		t.Fatalf("stop requirement lost: %v", err)
	}
	t.Setenv("EDEN_DESKTOP_DRIVER_MODE", "failure")
	if err := backend.PrepareUpdate(context.Background(), true); err == nil || err.Error() != "fixture failure" {
		t.Fatalf("prepare failure lost: %v", err)
	}
	t.Setenv("EDEN_DESKTOP_DRIVER_MODE", "block")
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := backend.Update(ctx, false, func(appupdate.State) {}); err == nil {
		t.Fatal("cancelled update helper accepted")
	}
}

func TestBackendStartupAndCancellation(t *testing.T) {
	t.Setenv("EDEN_DESKTOP_FIXTURE", "1")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	backend, err := NewBackend(executable, filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	backend.Directory = root
	for _, mode := range []string{"slow", "failure", "overflow", "invalid", "cancel", "deadline"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			deadline := time.Duration(0)
			fixture := mode
			if mode == "deadline" {
				deadline = 50 * time.Millisecond
				fixture = "cancel"
			}
			if mode == "cancel" {
				time.AfterFunc(50*time.Millisecond, cancel)
			}
			var result map[string]any
			started := time.Now()
			code, err := backend.JSON(ctx, deadline, 1024, &result, "-test.run=^TestBackendProcessFixture$", "--", fixture)
			switch mode {
			case "slow":
				if err != nil || code != 0 || result["status"] != "ready" || time.Since(started) < 200*time.Millisecond {
					t.Fatalf("startup readiness lost: %d %v %+v", code, err, result)
				}
			case "failure":
				if err != nil || code != 7 {
					t.Fatalf("helper exit lost: %d %v", code, err)
				}
			default:
				if err == nil {
					t.Fatalf("expected %s failure", mode)
				}
			}
			if time.Since(started) > 5*time.Second {
				t.Fatal("helper was not reaped within bounded time")
			}
		})
	}
}

func TestBackendEnvironmentAndBoundedOutput(t *testing.T) {
	root := t.TempDir()
	backend, err := NewBackend(filepath.Join(root, "resources", "eden.exe"), filepath.Join(root, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	command := backend.Command(context.Background(), "desktop", "--json")
	joined := strings.Join(command.Environ(), "\n")
	if !strings.Contains(joined, "EDEN_CONFIG="+backend.ConfigPath) || !strings.Contains(joined, "EDEN_DESKTOP_OWNER_PID=") {
		t.Fatal("backend profile/owner environment missing")
	}
	buffer := boundedOutput{limit: 4}
	_, err = buffer.Write([]byte("12345"))
	if err == nil || !buffer.exceeded || buffer.String() != "1234" {
		t.Fatal("output limit was not enforced")
	}
	var reply bootstrapReply
	if err = json.Unmarshal([]byte(`{"status":"ready","url":"http://localhost:1234/#secret"}`), &reply); err != nil || reply.Status != "ready" {
		t.Fatal("bootstrap contract changed")
	}
}
