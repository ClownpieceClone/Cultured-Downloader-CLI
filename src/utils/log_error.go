package utils

import (
	"os"
	"fmt"
	"log"
	"time"
	"sync"
	"path/filepath"

	"github.com/fatih/color"
)

var logMut sync.Mutex
var logFilePath = filepath.Join(
	APP_PATH, 
	"logs",
	fmt.Sprintf("cultured_downloader-cli_v%s_%s.log", VERSION, time.Now().Format("2006-01-02")),
)
// Thread-safe logging function that logs to "cultured_downloader.log" in the logs directory
func LogError(err error, errorMsg string, exit bool) {
	logMut.Lock()
	defer logMut.Unlock()

	if err == nil && errorMsg == "" {
		return
	}

	// write to log file
	f, fileErr := os.OpenFile(
		logFilePath, 
		os.O_WRONLY|os.O_CREATE|os.O_APPEND, 
		0666,
	)
	if fileErr != nil {
		fileErr = fmt.Errorf(
			"error opening log file: %v\nlog file path: %s", 
			fileErr, 
			logFilePath,
		)
		log.Println(color.RedString(fileErr.Error()))
		return
	}
	defer f.Close()

	// From https://www.golangprograms.com/get-current-date-and-time-in-various-format-in-golang.html
	now := time.Now().Format("2006-01-02 15:04:05")
	if err != nil && errorMsg != "" {
		fmt.Fprintf(f, "%v: %v\n", now, err)
		if errorMsg != "" {
			fmt.Fprintf(f, "Additional info: %v\n\n", errorMsg)
		}
	} else if err != nil {
		fmt.Fprintf(f, "%v: %v\n\n", now, err)
	} else {
		fmt.Fprintf(f, "%v: %v\n\n", now, errorMsg)
	}

	if exit {
		if err != nil {
			color.Red(err.Error())
		} else {
			color.Red(errorMsg)
		}
		os.Exit(1)
	}
}

// Uses the thread-safe LogError() function to log a slice of errors or a channel of errors
func LogErrors(exit bool, errChan *chan error, errs ...error) {
	if errChan != nil && len(errs) > 0 {
		panic(
			fmt.Sprintf(
				"error %d: cannot pass both an error channel and a slice of errors to LogErrors()",
				DEV_ERROR,
			),
		)
	}

	if errChan != nil {
		for err := range *errChan {
			LogError(err, "", exit)
		}
		return
	}
	for _, err := range errs {
		LogError(err, "", exit)
	}
}
