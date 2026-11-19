package main

import (
	"io"
	"net"
	"log"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		log.Fatal(err)
	}
	response, err := io.ReadAll(conn)
	conn.Close()
	if (err != nil) {
		log.Fatal(err)
	}
	if (string(response) != "OK\n") {
		log.Fatal(response)
	}
	log.Print(response)
}