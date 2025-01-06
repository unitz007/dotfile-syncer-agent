package main

import (
	"fmt"
	"strconv"
	"time"
)

func Infoln(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Println(time.Now().Format(time.RFC3339), "INFO:", s)
}

func Info(v ...string) {
	s := ""
	for _, c := range v {
		s = s + " " + c
	}

	fmt.Print(time.Now().Format(time.RFC3339), " INFO:", s)
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
