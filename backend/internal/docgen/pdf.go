package docgen

import (
	"context"
	"encoding/base64"
	"os"
	"time"

	"github.com/chromedp/chromedp"
)

func GeneratePDF(htmlBytes []byte) ([]byte, error) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("no-sandbox", true),
		chromedp.Flag("disable-gpu", true),
	)
	if p := os.Getenv("CHROME_PATH"); p != "" {
		opts = append(opts, chromedp.ExecPath(p))
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
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
