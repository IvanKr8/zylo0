package ui

import (
	"fmt"
	"io"
	"time"
)

// WithSpinner displays an animated spinner while executing function fn
// Uses writer for output, typically os.Stdout or tty file
func WithSpinner(writer io.Writer, message string, fn func() error) error {
	done := make(chan struct{})

	// Start spinner in background
	go func() {
		spin := []rune{'|', '/', '-', '\\'}
		i := 0
		for {
			select {
			case <-done:
				// Clear line and print completion
				fmt.Fprintf(writer, "\r\033[K%s done\n", message)
				return
			default:
				fmt.Fprintf(writer, "\r\033[K%s %c", message, spin[i%4])
				i++
				time.Sleep(120 * time.Millisecond)
			}
		}
	}()

	err := fn()
	close(done)

	if err != nil {
		// Clear line and show error
		fmt.Fprintf(writer, "\r\033[K%s failed: %v\n", message, err)
		return err
	}

	return nil
}
