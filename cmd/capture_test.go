package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeTestStateFile(t *testing.T, dir string, ts int64, snippet string) string {
	t.Helper()
	path := filepath.Join(dir, correctionPendingFile)
	content := fmt.Sprintf("%d\n%s\n", ts, snippet)
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatalf("writing state file: %v", err)
	}
	return path
}

func captureWithInput(t *testing.T, toolName string, command string, stateDir string) string {
	t.Helper()

	input := fmt.Sprintf(`{"tool_name":%q,"tool_input":{"command":%q},"tool_response":{}}`, toolName, command)

	// Redirect stdin
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.Write([]byte(input))
	w.Close()
	os.Stdin = r
	defer func() { os.Stdin = oldStdin }()

	// Capture stdout
	oldStdout := os.Stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW
	defer func() { os.Stdout = oldStdout }()

	// Override state file path for testing
	origFunc := correctionStatePathFunc
	correctionStatePathFunc = func() string {
		return filepath.Join(stateDir, correctionPendingFile)
	}
	defer func() { correctionStatePathFunc = origFunc }()

	Capture(nil, "")

	outW.Close()
	var buf bytes.Buffer
	buf.ReadFrom(outR)
	return buf.String()
}

func TestCaptureBashWithEngramStore(t *testing.T) {
	dir := t.TempDir()
	writeTestStateFile(t, dir, time.Now().Unix(), "test prompt")

	output := captureWithInput(t, "Bash", "engram store \"some fact\" --scope global", dir)

	if output != "" {
		t.Errorf("expected no output when engram store found, got: %q", output)
	}

	stateFile := filepath.Join(dir, correctionPendingFile)
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("expected state file to be deleted after engram store")
	}
}

func TestCaptureBashWithoutStore_PendingFresh(t *testing.T) {
	dir := t.TempDir()
	writeTestStateFile(t, dir, time.Now().Unix(), "user said actually X not Y")

	output := captureWithInput(t, "Bash", "ls -la", dir)

	if output == "" {
		t.Error("expected reminder output when correction pending but not stored")
	}
	if !bytes.Contains([]byte(output), []byte("engram reminder")) {
		t.Errorf("expected output to contain 'engram reminder', got: %q", output)
	}

	stateFile := filepath.Join(dir, correctionPendingFile)
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("expected state file to be deleted after reminder")
	}
}

func TestCaptureBashWithoutStore_NoStateFile(t *testing.T) {
	dir := t.TempDir()
	// No state file created

	output := captureWithInput(t, "Bash", "ls -la", dir)

	if output != "" {
		t.Errorf("expected no output when no state file, got: %q", output)
	}
}

func TestCaptureBashWithoutStore_StaleStateFile(t *testing.T) {
	dir := t.TempDir()
	// 10 minutes ago = stale
	writeTestStateFile(t, dir, time.Now().Unix()-600, "old correction")

	output := captureWithInput(t, "Bash", "ls -la", dir)

	if output != "" {
		t.Errorf("expected no output for stale state file, got: %q", output)
	}

	stateFile := filepath.Join(dir, correctionPendingFile)
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("expected stale state file to be deleted")
	}
}

func TestCaptureNonBashTool(t *testing.T) {
	dir := t.TempDir()
	writeTestStateFile(t, dir, time.Now().Unix(), "test prompt")

	output := captureWithInput(t, "Read", "", dir)

	if output != "" {
		t.Errorf("expected no output for non-Bash tool, got: %q", output)
	}

	// State file should still exist
	stateFile := filepath.Join(dir, correctionPendingFile)
	if _, err := os.Stat(stateFile); os.IsNotExist(err) {
		t.Error("state file should not be deleted for non-Bash tool")
	}
}

func TestCaptureBashWithEngramDelete(t *testing.T) {
	dir := t.TempDir()
	writeTestStateFile(t, dir, time.Now().Unix(), "test prompt")

	output := captureWithInput(t, "Bash", "engram delete 5", dir)

	if output != "" {
		t.Errorf("expected no output when engram delete found, got: %q", output)
	}

	stateFile := filepath.Join(dir, correctionPendingFile)
	if _, err := os.Stat(stateFile); !os.IsNotExist(err) {
		t.Error("expected state file to be deleted after engram delete")
	}
}
