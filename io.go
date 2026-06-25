package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ANSI colour codes
const (
	reset  = "\033[0m"
	dim    = "\033[2m"
	bold   = "\033[1m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	red    = "\033[31m"
	green  = "\033[32m"
)

func ts() string {
	return dim + time.Now().Format("15:04:05") + reset
}

func join(v []string) string {
	return strings.Join(v, " ")
}

func Infoln(v ...string) {
	fmt.Printf("%s  %s✦%s  %s\n", ts(), cyan, reset, join(v))
}

func Info(v ...string) {
	fmt.Printf("%s  %s✦%s  %s", ts(), cyan, reset, join(v))
}

func Warnln(v ...string) {
	fmt.Printf("%s  %s⚠️%s  %s\n", ts(), yellow, reset, join(v))
}

func Error(v ...string) {
	fmt.Printf("%s  %s✖%s  %s\n", ts(), red+bold, reset, join(v))
}

func Successln(v ...string) {
	fmt.Printf("%s  %s✔%s  %s\n", ts(), green+bold, reset, join(v))
}

// ConsoleSyncConsumer prints sync progress to the console.
func ConsoleSyncConsumer(event SyncEvent) {
	data := event.Data
	if data.Progress == 0 {
		Info("Sync triggered ===(0%)")
		time.Sleep(time.Second)
	} else {
		fmt.Print("===(" + strconv.Itoa(data.Progress) + "%)")
	}

	if !data.IsSuccess {
		time.Sleep(time.Second)
		fmt.Printf("=== failed: '%s' [%s]\n", data.Step, data.Error)
		return
	}

	if data.Done {
		time.Sleep(time.Second)
		fmt.Println("=== completed")
	}
}
