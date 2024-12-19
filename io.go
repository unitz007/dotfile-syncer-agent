package main

import (
	"fmt"
	"strconv"
	"time"
)

func Info(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Println(time.Now().Format(time.RFC3339), "INFO:", s)
}

func Error(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Println(time.Now().Format(time.RFC3339), "ERROR:", s)
}

func ConsoleSyncConsumer(event SyncEvent) {
	data := event.Data
	status := "===completed"
	if data.Progress == 25 {
		fmt.Print("Sync triggered===(0%)")
	} else {
		fmt.Print("===(" + strconv.Itoa(data.Progress) + "%)")
	}

	if !data.IsSuccess {
		msg := fmt.Sprintf("'%s': [%s]", data.Step, data.Error)
		status = fmt.Sprintf("===failed (%s)", msg)
		return
	}

	if data.Done {
		fmt.Printf("%s\n", status)
	}
}
