package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// GitHub's release CDN throttles each connection (about 80 KB/s on some networks),
// so a large download is split into ranges fetched in parallel.
const (
	downloadParts   = 8
	minParallelSize = 1 << 20
)

// DownloadToFile writes url into dst and rejects anything larger than maxSize.
// When the server supports range requests, a file of at least 1 MB downloads as
// parallel ranges; otherwise it streams as a single response.
func DownloadToFile(client *http.Client, url string, dst *os.File, maxSize int64, onProgress ProgressFunc) error {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	// A server without range support ignores this and sends the whole file with 200.
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		limited := io.LimitReader(NewProgressReader(resp.Body, resp.ContentLength, onProgress), maxSize+1)
		n, err := io.Copy(dst, limited)
		if err != nil {
			return err
		}
		if n > maxSize {
			return tooLarge(maxSize)
		}
		return nil
	case http.StatusPartialContent:
		total, err := rangeTotal(resp.Header.Get("Content-Range"))
		if err != nil {
			return err
		}
		if total > maxSize {
			return tooLarge(maxSize)
		}
		parts := downloadParts
		if total < minParallelSize {
			parts = 1
		}
		// Ranges go to the URL after redirects: the signed CDN URL, not github.com again.
		return downloadRanges(client, resp.Request.URL.String(), dst, total, parts, onProgress)
	default:
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}
}

func tooLarge(maxSize int64) error {
	return fmt.Errorf("download exceeds the maximum size (%d MB)", maxSize/(1024*1024))
}

// rangeTotal reads the full size from a Content-Range header such as "bytes 0-0/9665580".
func rangeTotal(header string) (int64, error) {
	_, size, ok := strings.Cut(header, "/")
	total, err := strconv.ParseInt(size, 10, 64)
	if !ok || err != nil || total <= 0 {
		return 0, fmt.Errorf("download returned an invalid Content-Range %q", header)
	}
	return total, nil
}

func downloadRanges(client *http.Client, url string, dst *os.File, total int64, parts int, onProgress ProgressFunc) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	progress := &sharedProgress{total: total, fn: onProgress}

	var wg sync.WaitGroup
	var once sync.Once
	var firstErr error
	size := (total + int64(parts) - 1) / int64(parts)
	for start := int64(0); start < total; start += size {
		end := min(start+size, total) - 1
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := downloadRange(ctx, rangeClient(client), url, dst, start, end, progress); err != nil {
				once.Do(func() { firstErr = err; cancel() })
			}
		}()
	}
	// Wait for every part, so no write lands after the caller removes a failed file.
	wg.Wait()
	return firstErr
}

// rangeClient copies client with a clone of its transport, so each range gets its
// own connection: over HTTP/2 the ranges would share one, and the throttle is per
// connection. The clone keeps the caller's TLS, proxy and dialer settings. A
// transport that cannot be cloned is shared as is.
func rangeClient(client *http.Client) *http.Client {
	c := *client
	switch t := client.Transport.(type) {
	case nil:
		c.Transport = http.DefaultTransport.(*http.Transport).Clone()
	case *http.Transport:
		c.Transport = t.Clone()
	}
	return &c
}

func downloadRange(ctx context.Context, client *http.Client, url string, dst *os.File, start, end int64, progress *sharedProgress) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	if t, ok := client.Transport.(*http.Transport); ok {
		defer t.CloseIdleConnections()
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("download of bytes %d-%d failed with status %d", start, end, resp.StatusCode)
	}

	want := end - start + 1
	body := io.LimitReader(resp.Body, want+1)
	n, err := io.Copy(io.NewOffsetWriter(dst, start), &countingReader{r: body, add: progress.add})
	if err != nil {
		return err
	}
	if n != want {
		return fmt.Errorf("download of bytes %d-%d returned %d bytes", start, end, n)
	}
	return nil
}

type countingReader struct {
	r   io.Reader
	add func(int64)
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	c.add(int64(n))
	return n, err
}

// sharedProgress sums the parts and reports like NewProgressReader: at most once
// per progressInterval, plus a final report when every byte has arrived.
type sharedProgress struct {
	mu       sync.Mutex
	read     int64
	total    int64
	fn       ProgressFunc
	lastSent time.Time
}

func (p *sharedProgress) add(n int64) {
	if p.fn == nil || n == 0 {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.read += n
	if p.read == p.total || time.Since(p.lastSent) >= progressInterval {
		p.lastSent = time.Now()
		p.fn(p.read, p.total)
	}
}
