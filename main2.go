package main

import (
	_ "bufio"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
)

func main2() {
	LoggerTest()
}

func LoggerTest() {
	stdout := os.Stdout
	logc := make(chan string, 9)
	logchannel := CreateChannelLogger(logc)
	var log_wg sync.WaitGroup
	log_wg.Add(1)
	go func() {
		defer log_wg.Done()
		LogChannel(stdout, logc)
	}()
	fmt.Fprintln(logchannel, "Starting the process!")
	var wg sync.WaitGroup
	for i := range 9 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			SomeLogThings(logchannel, i)
		}()
	}
	wg.Wait()
	// time.Sleep(250 * time.Millisecond)
	fmt.Fprintln(logchannel, "Completed distributed logging!")
	close(logc)
	log_wg.Wait()
	// fmt.Fprintln(stdout, "Completed distributed logging!")

}

func SomeLogThings(writer io.Writer, ct int) {
	fmt.Fprintln(writer, "Logging on goroutine ", ct)
	time.Sleep(1)
	fmt.Fprintln(writer, "Completed logging on goroutine ", ct)
}

type ChannelLogger struct {
	sender chan<- string
}

func (l ChannelLogger) Write(p []byte) (n int, err error) {
	l.sender <- string(p)
	return 0, nil
}

func CreateChannelLogger(sender chan<- string) io.Writer {
	return ChannelLogger{sender: sender}
}

func LogChannel(writer io.Writer, rcv <-chan string) {
	for log := range rcv {
		writer.Write([]byte(log))
	}
}
