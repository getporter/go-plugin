//go:build !windows
// +build !windows

package plugin

import (
	"context"
	"os"
	"reflect"
	"syscall"
	"testing"
	"time"
)

func TestClient_testInterfaceReattach(t *testing.T) {
	ctx := context.Background()

	// Setup the process for daemonization
	process := helperProcess("test-interface-daemon")
	if process.SysProcAttr == nil {
		process.SysProcAttr = &syscall.SysProcAttr{}
	}
	process.SysProcAttr.Setsid = true
	syscall.Umask(0)

	c := NewClient(&ClientConfig{
		Cmd:             process,
		HandshakeConfig: testHandshake,
		Plugins:         testPluginMap,
	})

	// Start it so we can get the reattach info
	if _, err := c.Start(ctx); err != nil {
		t.Fatalf("err should be nil, got %s", err)
	}

	// New client with reattach info
	reattach := c.ReattachConfig()
	if reattach == nil {
		c.Kill(ctx)
		t.Fatal("reattach config should be non-nil")
	}

	// Find the process and defer a kill so we know it is gone
	p, err := os.FindProcess(reattach.Pid)
	if err != nil {
		c.Kill(ctx)
		t.Fatalf("couldn't find process: %s", err)
	}
	defer p.Kill()

	// Reattach
	c = NewClient(&ClientConfig{
		Reattach:        reattach,
		HandshakeConfig: testHandshake,
		Plugins:         testPluginMap,
	})

	// Start shouldn't error
	if _, err := c.Start(ctx); err != nil {
		t.Fatalf("err: %s", err)
	}

	// It should still be alive
	time.Sleep(1 * time.Second)
	if c.Exited() {
		t.Fatal("should not be exited")
	}

	// Grab the RPC client
	client, err := c.Client(ctx)
	if err != nil {
		t.Fatalf("err should be nil, got %s", err)
	}

	// Grab the impl
	raw, err := client.Dispense("test")
	if err != nil {
		t.Fatalf("err should be nil, got %s", err)
	}

	impl, ok := raw.(testInterface)
	if !ok {
		t.Fatalf("bad: %#v", raw)
	}

	result := impl.Double(21)
	if result != 42 {
		t.Fatalf("bad: %#v", result)
	}

	// Test the resulting reattach config
	reattach2 := c.ReattachConfig()
	if reattach2 == nil {
		t.Fatal("reattach from reattached should not be nil")
	}
	if !reflect.DeepEqual(reattach, reattach2) {
		t.Fatalf("bad: %#v", reattach)
	}

	// Kill it
	c.Kill(ctx)

	// Test that it knows it is exited
	if !c.Exited() {
		t.Fatal("should say client has exited")
	}
}
