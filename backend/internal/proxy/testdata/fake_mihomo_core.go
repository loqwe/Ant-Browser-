package main

import (
	"net"
	"os"
	"strings"
)

func main() {
	cfgPath := ""
	for i := 0; i < len(os.Args)-1; i++ {
		if os.Args[i] == "-f" {
			cfgPath = os.Args[i+1]
			break
		}
	}
	if cfgPath == "" {
		os.Exit(2)
	}
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		os.Exit(3)
	}
	port := ""
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "socks-port:") {
			port = strings.TrimSpace(strings.TrimPrefix(line, "socks-port:"))
			break
		}
	}
	if port == "" {
		os.Exit(4)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		os.Exit(5)
	}
	defer ln.Close()
	for {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		_ = conn.Close()
	}
}
