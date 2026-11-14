package docgen

import (
	"context"
	"encoding/base64"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

// maxConcurrentPDFRenders caps how many Chromium processes may render at
// once; each one needs hundreds of MB.
const maxConcurrentPDFRenders = 2

// pdfRenderTimeout bounds one GeneratePDF call, including the wait for a
// render slot. It stays under the server's 60 s write timeout.
const pdfRenderTimeout = 30 * time.Second

// pdfSlots is a semaphore holding one token per running render.
var pdfSlots = make(chan struct{}, maxConcurrentPDFRenders)

// renderPDF does the Chromium work once a slot is held. Tests replace it to
// observe concurrency without a browser.
var renderPDF = renderPDFWithChrome

// GeneratePDF renders htmlBytes to an A4 PDF in a headless Chromium. At most
// maxConcurrentPDFRenders calls run Chromium at once; the rest wait for a
// slot. Cancelling ctx stops the wait or kills the browser and returns
// ctx's error.
//
// ctx is checked before and after taking a slot because select picks at
// random among ready cases: a cancelled request must never start a browser.
func GeneratePDF(ctx context.Context, htmlBytes []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case pdfSlots <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	defer func() { <-pdfSlots }()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return renderPDF(ctx, htmlBytes)
}

// renderPDFWithChrome starts a Chromium tied to ctx, so cancelling ctx kills
// the browser process.
func renderPDFWithChrome(ctx context.Context, htmlBytes []byte) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
	)
	if p := os.Getenv("CHROME_PATH"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(ctx, opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	dataURI := "data:text/html;base64," + base64.StdEncoding.EncodeToString(htmlBytes)

	if err := chromedp.Do(ctx, chromedp.Navigate(dataURI)); err != nil {
		return nil, err
	}

	pdfBuf, err := chromedp.Run(ctx, chromedp.PrintToPDF(
		chromedp.PDFPaper(chromedp.PaperA4),
		chromedp.PDFPrintBackground(),
	))
	if err != nil {
		return nil, err
	}

	return pdfBuf, nil
}
