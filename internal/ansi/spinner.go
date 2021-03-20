package ansi

import (
	"os"
	"time"

	"github.com/briandowns/spinner"
)

const (
	spinnerTextEllipsis = "..."
	spinnerTextDone     = "done"
	spinnerTextFailed   = "failed"

	spinnerColor = "blue"
)

func Waiting(fn func() error) error {
	return loading("", "", "", fn)
}

func Spinner(text string, fn func() error) error {
	initialMsg := text + spinnerTextEllipsis + " "
	doneMsg := initialMsg + spinnerTextDone
	failMsg := initialMsg + spinnerTextFailed

	return loading(initialMsg, doneMsg, failMsg, fn)
}

func loading(initialMsg, doneMsg, failMsg string, fn func() error) error {
	done := make(chan struct{})
	errc := make(chan error)
	go func() {
		defer close(done)

		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(os.Stderr))
		s.Prefix = initialMsg
		s.FinalMSG = doneMsg

		if err := s.Color(spinnerColor); err != nil {
			panic(err)
		}

		s.Start()
		err := <-errc
		if err != nil {
			s.FinalMSG = failMsg
		}

		s.Stop()
	}()

	err := fn()
	errc <- err
	<-done
	return err
}
