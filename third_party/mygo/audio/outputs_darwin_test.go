//go:build darwin

package audio

import (
	"runtime"
	"testing"
	"time"
)

// This Mac's own outputs: its speakers, a display's, an interface's. The
// test needs one; it skips where there is none, as it does on a machine
// without a sound card.
func TestOutputsListsTheSystemsOutputs(t *testing.T) {
	outs, err := listOutputs()
	if err != nil || len(outs) == 0 {
		t.Skipf("no outputs to list here: %v", err)
	}
	for _, o := range outs {
		if o.ID == "" || o.Name == "" {
			t.Errorf("output %+v has no ID or name", o)
		}
	}
	t.Logf("%d outputs, default first: %+v", len(outs), outs)
	if !outs[0].Default {
		t.Logf("the system's default is not among them, or is not listed first")
	}
}

// The queue plays through the output asked for, and through the default
// while that one is gone.
func TestQueuePlaysThroughTheOutputAskedFor(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("macOS's queues alone")
	}
	outs, err := listOutputs()
	if err != nil || len(outs) == 0 {
		t.Skipf("no outputs to play through here: %v", err)
	}
	want := outs[0].ID
	defer SetOutput("")

	SetOutput(want)
	d, _, err := openDevice(20 * time.Millisecond)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer d.close()
	if got := d.(*aqDevice).playing; got != want {
		t.Errorf("the queue plays through %q, want %q", got, want)
	}

	SetOutput("no such output")
	gone, _, err := openDevice(20 * time.Millisecond)
	if err != nil {
		t.Fatalf("open with the output gone: %v", err)
	}
	defer gone.close()
	if dev := gone.(*aqDevice); !dev.fellBack || dev.playing != "" {
		t.Errorf("a queue asked for an output that is gone plays through %q (fell back %v), want the default", dev.playing, dev.fellBack)
	}
}
