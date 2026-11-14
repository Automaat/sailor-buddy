package docgen

import (
	"bytes"
	"context"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func stubRenderPDF(t *testing.T, fn func(context.Context, []byte) ([]byte, error)) {
	t.Helper()
	orig := renderPDF
	renderPDF = fn
	t.Cleanup(func() { renderPDF = orig })
}

func TestGeneratePDF_CancelledContext(t *testing.T) {
	var calls atomic.Int32
	stubRenderPDF(t, func(context.Context, []byte) ([]byte, error) {
		calls.Add(1)
		return []byte("%PDF"), nil
	})

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	_, err := GeneratePDF(ctx, []byte("<p>x</p>"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Fatalf("cancelled call took %s", d)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("render ran %d times for a cancelled request", n)
	}
}

func TestGeneratePDF_CancelWhileWaitingForSlot(t *testing.T) {
	stubRenderPDF(t, func(context.Context, []byte) ([]byte, error) {
		t.Error("render must not run while all slots are taken")
		return nil, nil
	})
	for range maxConcurrentPDFRenders {
		pdfSlots <- struct{}{}
	}
	t.Cleanup(func() {
		for range maxConcurrentPDFRenders {
			<-pdfSlots
		}
	})

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()

	_, err := GeneratePDF(ctx, []byte("<p>x</p>"))
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want context.DeadlineExceeded, got %v", err)
	}
}

func TestGeneratePDF_BoundsConcurrentRenders(t *testing.T) {
	const callers = 5
	var inFlight, peak atomic.Int32
	entered := make(chan struct{}, callers)
	release := make(chan struct{})
	stubRenderPDF(t, func(context.Context, []byte) ([]byte, error) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		entered <- struct{}{}
		<-release
		return []byte("%PDF"), nil
	})

	var wg sync.WaitGroup
	errs := make(chan error, callers)
	for range callers {
		wg.Go(func() {
			_, err := GeneratePDF(t.Context(), []byte("<p>x</p>"))
			errs <- err
		})
	}

	for range maxConcurrentPDFRenders {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("renders did not start")
		}
	}
	select {
	case <-entered:
		t.Fatalf("more than %d renders started at once", maxConcurrentPDFRenders)
	case <-time.After(100 * time.Millisecond):
	}

	close(release)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("render failed: %v", err)
		}
	}
	if p := peak.Load(); p != maxConcurrentPDFRenders {
		t.Fatalf("peak concurrent renders = %d, want %d", p, maxConcurrentPDFRenders)
	}
}

func requireChrome(t *testing.T) {
	t.Helper()
	if os.Getenv("CHROME_PATH") == "" {
		t.Skip("CHROME_PATH not set")
	}
}

func TestGeneratePDF_Chrome(t *testing.T) {
	requireChrome(t)

	var inFlight, peak atomic.Int32
	stubRenderPDF(t, func(ctx context.Context, html []byte) ([]byte, error) {
		n := inFlight.Add(1)
		defer inFlight.Add(-1)
		for {
			p := peak.Load()
			if n <= p || peak.CompareAndSwap(p, n) {
				break
			}
		}
		return renderPDFWithChrome(ctx, html)
	})

	const callers = 5
	var wg sync.WaitGroup
	out := make(chan []byte, callers)
	errs := make(chan error, callers)
	for range callers {
		wg.Go(func() {
			pdf, err := GeneratePDF(t.Context(), []byte("<h1>Opinia</h1>"))
			if err != nil {
				errs <- err
				return
			}
			out <- pdf
		})
	}
	wg.Wait()
	close(out)
	close(errs)
	for err := range errs {
		t.Fatalf("render failed: %v", err)
	}
	for pdf := range out {
		if !bytes.HasPrefix(pdf, []byte("%PDF")) {
			t.Fatalf("output is not a PDF: %q", pdf[:min(len(pdf), 16)])
		}
	}
	if p := peak.Load(); p > maxConcurrentPDFRenders {
		t.Fatalf("peak concurrent Chromium renders = %d, want at most %d", p, maxConcurrentPDFRenders)
	}
}

// slowLoadHTML blocks the page's load event for 5 s, so a render is still
// navigating when a shorter deadline expires.
const slowLoadHTML = `<script>const end = Date.now() + 5000; while (Date.now() < end) {}</script><h1>Opinia</h1>`

func TestGeneratePDF_ChromeCancelMidRender(t *testing.T) {
	requireChrome(t)

	ctx, cancel := context.WithTimeout(t.Context(), 1500*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := GeneratePDF(ctx, []byte(slowLoadHTML))
	if err == nil {
		t.Fatal("render outlived its context deadline")
	}
	if !errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled) {
		t.Fatalf("want a context error, got %v", err)
	}
	if d := time.Since(start); d > 4*time.Second {
		t.Fatalf("cancelled render took %s; the browser was not stopped", d)
	}
}
