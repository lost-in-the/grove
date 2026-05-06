package docker

import (
	"testing"
)

func TestParseServiceHealth_Running(t *testing.T) {
	jsonOut := `[{"Service":"app","State":"running","ExitCode":0,"Health":""}]`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "app" || got[0].Status != ServiceRunning {
		t.Errorf("got %#v", got)
	}
}

func TestParseServiceHealth_ExitedSuccess(t *testing.T) {
	jsonOut := `[{"Service":"asset_precompile","State":"exited","ExitCode":0,"Health":""}]`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Status != ServiceExitedClean {
		t.Errorf("expected ServiceExitedClean, got %v", got[0].Status)
	}
}

func TestParseServiceHealth_ExitedFailed(t *testing.T) {
	jsonOut := `[{"Service":"db_seed","State":"exited","ExitCode":1,"Health":""}]`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got[0].Status != ServiceExitedError {
		t.Errorf("expected ServiceExitedError, got %v", got[0].Status)
	}
}

func TestParseServiceHealth_NDJSON(t *testing.T) {
	// Some compose versions emit one JSON object per line rather than an array
	jsonOut := `{"Service":"app","State":"running","ExitCode":0}
{"Service":"db","State":"running","ExitCode":0}`
	got, err := parseServiceHealth([]byte(jsonOut))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 services, got %d: %#v", len(got), got)
	}
}

func TestClassifyHealth_NonBlockingExitedTreatedAsOK(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
		{Name: "asset_precompile", Status: ServiceExitedError},
	}
	healthy, blockers := classifyHealth(statuses, []string{"asset_precompile"})
	if !healthy {
		t.Errorf("expected healthy=true (asset_precompile is non-blocking), got false. blockers: %v", blockers)
	}
	if len(blockers) != 0 {
		t.Errorf("expected no blockers, got %v", blockers)
	}
}

func TestClassifyHealth_BlockingFailedReportsBlocker(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceExitedError},
		{Name: "asset_precompile", Status: ServiceExitedError},
	}
	healthy, blockers := classifyHealth(statuses, []string{"asset_precompile"})
	if healthy {
		t.Errorf("expected unhealthy (app is blocking), got healthy")
	}
	if len(blockers) != 1 || blockers[0] != "app" {
		t.Errorf("expected blockers=[app], got %v", blockers)
	}
}

func TestClassifyHealth_NonBlockingRunningIsNormal(t *testing.T) {
	statuses := []ServiceStatus{
		{Name: "app", Status: ServiceRunning},
	}
	healthy, _ := classifyHealth(statuses, nil)
	if !healthy {
		t.Errorf("expected healthy")
	}
}
