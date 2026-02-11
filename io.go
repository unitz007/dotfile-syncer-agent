package main

import (
	"fmt"
	"strconv"
	"time"
)

// Infoln prints an informational log message with timestamp to stdout, followed by a newline.
// All arguments are concatenated with spaces between them.
func Infoln(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Println(time.Now().Format(time.RFC3339), "INFO:", s)
}

// Info prints an informational log message with timestamp to stdout without a newline.
// All arguments are concatenated with spaces between them.
func Info(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Print(time.Now().Format(time.RFC3339), " INFO:", s)
}

// Error prints an error log message with timestamp to stdout.
// All arguments are concatenated with spaces between them.
func Error(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Println(time.Now().Format(time.RFC3339), "ERROR:", s)
}

// ConsoleSyncConsumer is a Consumer implementation that prints sync progress to the console.
// It displays progress as a percentage and shows completion or error status.
func ConsoleSyncConsumer(event SyncEvent) {
	data := event.Data
	status := "===completed"
	if data.Progress == 0 {
		Info("Sync triggered===(0%)")
		time.Sleep(time.Second)
	} else {
		fmt.Print("===(" + strconv.Itoa(data.Progress) + "%)")
	}

	if !data.IsSuccess {
		time.Sleep(time.Second)
		msg := fmt.Sprintf("'%s': [%s]", data.Step, data.Error)
		status = fmt.Sprintf("===failed (%s)", msg)
		fmt.Printf("%s\n", status)
		return
	}

	if data.Done {
		time.Sleep(time.Second)
		fmt.Printf("%s\n", status)
	}
}
