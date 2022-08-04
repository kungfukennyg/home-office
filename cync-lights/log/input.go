package log

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"
)

func ListenForInputAsync() (chan string, chan<- struct{}) {
	send := make(chan string)
	closeCh := make(chan struct{})
	go func(ch chan string) {
		for {
			input := bufio.NewReader(os.Stdin)
			str, err := input.ReadString('\n')
			if err != nil && err != io.EOF {
				fmt.Printf("failed to read stdin: %v\b", err)
				continue
			}

			var closeChannel bool
		outer:
			for {
				select {
				case _, ok := <-closeCh:
					if ok {
						closeChannel = true
					}
				case <-time.After(500 * time.Millisecond):
					break outer
				}
			}
			if closeChannel {
				close(closeCh)
				return
			} else {
				ch <- str
			}
		}
	}(send)

	return send, closeCh
}
