package ui

import (
	"fmt"
	"os"
	"time"
)

func WithSpinner(tty *os.File, message string, fn func() error) error {
	writer := tty

	fmt.Fprintf(writer, "%s ", message)

	done := make(chan struct{})

	go func() {
		spin := []rune{'|', '/', '-', '\\'}
		i := 0
		for {
			select {
			case <-done:
				return
			default:
				fmt.Fprintf(writer, "\r\033[K%s %c", message, spin[i%4])
				time.Sleep(120 * time.Millisecond)
				i++
			}
		}
	}()

	err := fn()
	close(done)

	if err != nil {
		fmt.Fprintf(writer, "\r\033[KError: %v\n", err)
		return err
	}

	fmt.Fprintf(writer, "\r\033[K%s done\n", message)
	return nil
}
