package main

import (
	"os"
	"testing"

	"github.com/spezifisch/stmps/logger"
	"github.com/spezifisch/stmps/mpvplayer"
	"github.com/stretchr/testify/assert"
)

// Test initialization of the player
func TestPlayerInitialization(t *testing.T) {
	logger := logger.Init()
	player, err := mpvplayer.NewPlayer(logger)
	assert.NoError(t, err, "Player initialization should not return an error")
	assert.NotNil(t, player, "Player should be initialized")
}

func TestMainWithoutTUI(t *testing.T) {
	// Mock osExit to prevent actual exit during test
	exitCalled := false
	osExit = func(code int) {
		exitCalled = true
		if code != 0 {
			t.Fatalf("Unexpected exit with code: %d", code)
		}
		// Since we don't abort execution here, we will run main() until the end or a panic.
	}
	headlessMode = true

	// Restore osExit after the test
	defer func() {
		osExit = os.Exit
		headlessMode = false
	}()

	// Set command-line arguments to trigger the help flag
	os.Args = []string{"cmd", "--help"}

	main()

	if !exitCalled {
		t.Fatalf("osExit was not called")
	}
}
