package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/k33alexey/MetaLab/internal/appconfig"
	"github.com/k33alexey/MetaLab/internal/metadata"
	"github.com/k33alexey/MetaLab/internal/platform"
	"github.com/k33alexey/MetaLab/internal/uuid"
)

// Every stand scripts/test_stand.py makes reads whole, as the test cases
// expect of it (docs/test-cases/blok-2v.md, «Стенды»): a rule of the model
// that the script does not follow leaves the stand unread, and every case on
// it fails at the first step. Defect caught: a function of a source with no
// type, which the script wrote and the model refuses since 2.555, found
// only when the cases of block 2б were walked.
func TestTheStandsOfTheTestCasesRead(t *testing.T) {
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("python3 is not on the path")
	}
	for _, level := range []string{"c1", "c2", "c3"} {
		t.Run(level, func(t *testing.T) {
			folder := filepath.Join(t.TempDir(), "stand")
			made, err := exec.Command("python3", "../test_stand.py", folder, level).CombinedOutput()
			if err != nil {
				t.Fatalf("test_stand.py %s: %v: %s", level, err, made)
			}
			if _, err := metadata.Load(folder); err != nil {
				t.Fatalf("stand %s does not read: %v", level, err)
			}
		})
	}
}

// up and down touch nothing in a folder that holds no stand: up refuses a
// folder with something in it, down refuses a folder with no stand and does
// not make one. Defects caught (review 2.464, П2): down removing, with the
// stand, the files that lay in the folder before up made it there; down on a
// folder with no stand making the folder and its lock before it refused.
func TestAFolderWithNoStandIsLeftAsItWas(t *testing.T) {
	ctx := context.Background()
	foreign := t.TempDir()
	kept := filepath.Join(foreign, "notes.txt")
	if err := os.WriteFile(kept, []byte("before the stand"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := up(ctx, foreign, t.TempDir(), 8190); err == nil || !strings.Contains(err.Error(), "holds no stand") {
		t.Errorf("up on a folder with a file in it and no stand: %v", err)
	}
	if err := down(ctx, foreign); err == nil || !strings.Contains(err.Error(), "holds no stand") {
		t.Errorf("down on a folder with a file in it and no stand: %v", err)
	}
	entries, err := os.ReadDir(foreign)
	if err != nil || len(entries) != 1 || entries[0].Name() != "notes.txt" {
		t.Errorf("the folder after up and down holds %v (%v), want notes.txt alone", entries, err)
	}
	missing := filepath.Join(t.TempDir(), "never")
	if err := down(ctx, missing); err == nil {
		t.Error("down on a folder that is not there did not refuse")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("down on a folder that is not there made it: %v", err)
	}
}

// The stand's page serves requests at once without sharing what it reads:
// the list of accounts is read anew by each request into its own copy, and
// opening Studio reads the administrator's password beside it. Run under
// -race, as make ci runs it. Defect caught (review 2.464, П3): every request
// read stand.json into the one stand of up, so two requests at once wrote
// and read one map - concurrent map writes, which takes the stand with ML
// App and ML Manager down.
func TestTheStandsPageServesRequestsAtOnce(t *testing.T) {
	t.Setenv("ML_SYSTEM_DATABASE_URL", "")
	folder := t.TempDir()
	stand := state{Project: folder, Database: uuid.MustNew(), Port: 8190, Passwords: map[string]string{administrator: standPassword(administrator)}}
	if err := writeJSON(filepath.Join(folder, "stand.json"), stand); err != nil {
		t.Fatal(err)
	}
	// No ML System: opening Studio stops at signing in, after the password
	// is read.
	runtime := platform.New(context.Background(), appconfig.Default(), nil)
	defer runtime.Close()
	page := standPage(folder, stand, "127.0.0.1:8191", "127.0.0.1:8192", runtime, &studioWindows{root: context.Background(), runtime: runtime})
	var wait sync.WaitGroup
	for index := 0; index < 40; index++ {
		for _, path := range []string{"/", "/studio"} {
			wait.Add(1)
			go func() {
				defer wait.Done()
				response := httptest.NewRecorder()
				page.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				if path == "/" && !strings.Contains(response.Body.String(), "admin-stand-c5") {
					t.Errorf("the page shows no account: %s", response.Body.String())
				}
			}()
		}
	}
	wait.Wait()
}
