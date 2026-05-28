package main

import (
	"io"
	"log"
	"net"
	"os"
)

func main() {
	target := os.Getenv("PG_TARGET") // "postgres:5432"
	if target == "" {
		target = "postgres:5432"
	}
	listener, err := net.Listen("tcp", ":6543")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	log.Println("DB Proxy listening on :6543, forwarding to", target)
	for {
		client, err := listener.Accept()
		if err != nil {
			log.Println(err)
			continue
		}
		go handle(client, target)
	}
}

func handle(client net.Conn, target string) {
	defer client.Close()
	server, err := net.Dial("tcp", target)
	if err != nil {
		log.Println("Cannot connect to target:", err)
		return
	}
	defer server.Close()
	go io.Copy(server, client)
	io.Copy(client, server)
}
