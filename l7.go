// go-stress.go — HTTP stress tester with robust, idiomatic client usage.
// Usage: go run go-stress.go <URL> <THREADS> <DURATION_SEC> [CUSTOM_HOST]

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	// DNS state
	ips      []string
	ipsMutex sync.Mutex
)

// StressConfig holds parsed CLI values.
type StressConfig struct {
	Target         *url.URL
	Threads        int
	Duration       time.Duration
	RequestTimeout time.Duration
	CustomHost     string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// randomIP returns a pseudorandom IPv4 string.
func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d",
		rand.Intn(256), rand.Intn(256),
		rand.Intn(256), rand.Intn(256))
}

// randomString returns a random alphanumeric string of length n.
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// randomUserAgent picks Chrome or Firefox on random OS.
func randomUserAgent() string {
	oses := []string{
		"Windows NT 10.0; Win64; x64",
		"Macintosh; Intel Mac OS X 10_15_7",
		"X11; Linux x86_64",
	}
	osPart := oses[rand.Intn(len(oses))]

	if rand.Intn(2) == 0 {
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%d.0.%d.%d Safari/537.36",
			osPart, rand.Intn(30)+90, rand.Intn(4000), rand.Intn(200))
	}
	return fmt.Sprintf("Mozilla/5.0 (%s; rv:%d.0) Gecko/20100101 Firefox/%d.%d",
		osPart, rand.Intn(30)+70, rand.Intn(30)+70, rand.Intn(10))
}

// randomLanguage returns a plausible Accept-Language header.
func randomLanguage() string {
	langs := []string{
		"en-US,en;q=0.9",
		"en-GB,en;q=0.8",
		"fr-FR,fr;q=0.9,en-US;q=0.8",
		"de-DE,de;q=0.9,en-US;q=0.8",
	}
	return langs[rand.Intn(len(langs))]
}

// updateIPs atomically replaces the global IP list.
func updateIPs(newIPs []string) {
	ipsMutex.Lock()
	defer ipsMutex.Unlock()
	ips = newIPs
}

// snapshotIPs returns a copy of the current IP list.
func snapshotIPs() []string {
	ipsMutex.Lock()
	defer ipsMutex.Unlock()
	out := make([]string, len(ips))
	copy(out, ips)
	return out
}

// lookupIPv4 resolves and returns sorted IPv4 addresses for host.
func lookupIPv4(host string) ([]string, error) {
	addrs, err := net.LookupIP(host)
	if err != nil {
		return nil, err
	}
	var v4s []string
	for _, a := range addrs {
		if ip4 := a.To4(); ip4 != nil {
			v4s = append(v4s, ip4.String())
		}
	}
	if len(v4s) == 0 {
		return nil, fmt.Errorf("no IPv4 addresses found for %s", host)
	}
	sort.Strings(v4s)
	return v4s, nil
}

// dnsRefresher re-resolves host every interval, updates global IPs, signals rebalance.
func dnsRefresher(ctx context.Context, host string, interval time.Duration, rebalanceCh chan<- []string) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			newIPs, err := lookupIPv4(host)
			if err != nil {
				log.Printf("DNS lookup error for %s: %v", host, err)
				continue
			}
			updateIPs(newIPs)
			select {
			case rebalanceCh <- newIPs:
			default:
			}
		}
	}
}

// buildRequest constructs an *http.Request with dynamic headers.
func buildRequest(ctx context.Context, cfg *StressConfig) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", cfg.Target.String(), nil)
	if err != nil {
		return nil, err
	}
	if cfg.CustomHost != "" {
		req.Host = cfg.CustomHost
	}
	ua := randomUserAgent()
	h := req.Header
	h.Set("User-Agent", ua)
	h.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	h.Set("Accept-Language", randomLanguage())
	h.Set("DNT", "1")
	h.Set("Cache-Control", "no-cache")
	h.Set("X-Forwarded-For", randomIP())
	if strings.Contains(ua, "Chrome/") {
		// simple client hints example
		h.Add("Sec-CH-UA", `"Google Chrome";v="`+strings.Split(ua, "Chrome/")[1]+`"`)
		h.Add("Sec-CH-UA-Mobile", "?0")
		h.Add("Sec-CH-UA-Platform", `"`+randomString(6)+`"`)
	}
	return req, nil
}

// workerLoop continuously issues HTTP requests until ctx is done.
func workerLoop(ctx context.Context, cfg *StressConfig, client *http.Client, id int) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			// per-request timeout
			reqCtx, cancel := context.WithTimeout(ctx, cfg.RequestTimeout)
			req, err := buildRequest(reqCtx, cfg)
			if err == nil {
				if resp, err := client.Do(req); err == nil {
					// Drain up to 1 KiB to advance window
					io.CopyN(io.Discard, resp.Body, 1024)
					resp.Body.Close()
				}
			}
			cancel()
		}
	}
}

// runManager ensures exactly cfg.Threads workers; respawns on DNS changes.
func runManager(ctx context.Context, cfg *StressConfig) {
	rebalanceCh := make(chan []string, 1)
	go dnsRefresher(ctx, cfg.Target.Hostname(), 30*time.Second, rebalanceCh)

	// prepare HTTP client
	tr := &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		TLSHandshakeTimeout: 3 * time.Second,
		MaxIdleConnsPerHost: cfg.Threads,
		IdleConnTimeout:     30 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	client := &http.Client{
		Transport: tr,
		Timeout:   cfg.RequestTimeout,
	}

	var (
		mu      sync.Mutex
		workers = make(map[int]context.CancelFunc)
	)
	spawn := func(uid int) {
		wCtx, wCancel := context.WithCancel(ctx)
		mu.Lock()
		workers[uid] = wCancel
		mu.Unlock()
		go workerLoop(wCtx, cfg, client, uid)
	}
	cancelAll := func() {
		mu.Lock()
		for _, c := range workers {
			c()
		}
		mu.Unlock()
	}

	// initial DNS and spawn workers
	initial, err := lookupIPv4(cfg.Target.Hostname())
	if err != nil {
		log.Fatalf("Initial DNS lookup failed: %v", err)
	}
	updateIPs(initial)
	for i := 0; i < cfg.Threads; i++ {
		spawn(i)
	}

	for {
		select {
		case <-ctx.Done():
			cancelAll()
			return
		case <-rebalanceCh:
			// simple: cancel and respawn all threads
			cancelAll()
			for i := 0; i < cfg.Threads; i++ {
				spawn(i)
			}
		}
	}
}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <URL> <THREADS> <DURATION_SEC> [CUSTOM_HOST]\n", os.Args[0])
		os.Exit(1)
	}
	rawURL := os.Args[1]
	threads, err := strconv.Atoi(os.Args[2])
	if err != nil || threads <= 0 {
		log.Fatalf("Invalid THREADS (%q)", os.Args[2])
	}
	durSec, err := strconv.Atoi(os.Args[3])
	if err != nil || durSec <= 0 {
		log.Fatalf("Invalid DURATION_SEC (%q)", os.Args[3])
	}
	customHost := ""
	if len(os.Args) > 4 {
		customHost = os.Args[4]
	}

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Scheme == "" || parsed.Hostname() == "" {
		log.Fatalf("Invalid URL: %q", rawURL)
	}

	cfg := &StressConfig{
		Target:         parsed,
		Threads:        threads,
		Duration:       time.Duration(durSec) * time.Second,
		RequestTimeout: 5 * time.Second, // fixed per-request timeout
		CustomHost:     customHost,
	}

	// root context with timeout + SIGINT/SIGTERM handling
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("Signal received: shutting down early.")
		cancel()
	}()

	fmt.Printf("Starting stress test: %s | threads=%d | duration=%v\n",
		cfg.Target, cfg.Threads, cfg.Duration)
	runManager(ctx, cfg)
	fmt.Println("Stress test complete.")
}
