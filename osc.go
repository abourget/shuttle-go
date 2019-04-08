package main

import (
	"fmt"

	"github.com/hypebeast/go-osc/osc"
)

func listenOSCFeedback() {
	addr := "127.0.0.1:8000"
	server := &osc.Server{Addr: addr}

	server.Handle("*", func(msg *osc.Message) {
		osc.PrintMessage(msg)
	})

	fmt.Println("Listening on :8000 for incoming OSC feedback")
	server.ListenAndServe()
}
