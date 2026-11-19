package main

import (
	"net"
	"log"
)

func main() {
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatal(err)
	}
	defer listener.Close()
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("err with connection")
			return
		}

		log.Println("client connected")
		_, err = conn.Write([]byte("OK\n"))
		if err != nil {
			log.Println("err with ans")
		}
		conn.Close()
	}

}