package ansi

import (
	"fmt"
	"time"

	"github.com/briandowns/spinner"
)

const (
	spinnerTextEllipsis = "..."
	spinnerTextDone     = "done"
	spinnerTextFailed   = "failed"

	spinnerColor = "red"
)

func Spinner(text string, fn func() error) error {
	done := make(chan struct{})
	errc := make(chan error)
	go func() {
		defer close(done)

		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond)
		s.Prefix = text + spinnerTextEllipsis + " "
		s.FinalMSG = s.Prefix + spinnerTextDone

		if err := s.Color(spinnerColor); err != nil {
			panic(err)
		}

		s.Start()
		err := <-errc
		if err != nil {
			s.FinalMSG = s.Prefix + spinnerTextFailed
		}

		// FIXME(cyx): this is causing a race condition. The problem is
		// with our dependency on briandowns/spinner. For now adding an
		// artificial sleep removes the race condition.
		time.Sleep(time.Microsecond)

		s.Stop()
	}()

	err := fn()
	errc <- err
	<-done
	fmt.Println()
	return err
}
