package main

import (
	"fmt"

	"github.com/hypebeast/go-osc/osc"
)

func listenOSCFeedback() {
	d := osc.NewStandardDispatcher()
	err := d.AddMsgHandler("*", func(msg *osc.Message) {
		osc.PrintMessage(msg)
	})
	if err != nil {
		fmt.Printf("Error creating osc dispatcher for OSC feedback: %v\n", err)
		return
	}

	fmt.Println("Listening on :8000 for incoming OSC feedback")
	err = (&osc.Server{
		Addr:       "127.0.0.1:8000",
		Dispatcher: d,
	}).ListenAndServe()
	if err != nil {
		fmt.Printf("Error listening for OSC feedback: %v\n", err)
		return
	}
}
