package ansi

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"

	"github.com/briandowns/spinner"

	"github.com/auth0/auth0-cli/internal/auth0"
	"github.com/auth0/auth0-cli/internal/iostream"
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

// Spinner simulates a spinner animation while executing a function.
func Spinner(text string, fn func() error) error {
	// Spinner frames
	frames := []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"}

	// Styles
	spinnerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("45")) // Cyan
	textStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))              // Gray
	successStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42")) // Green
	failStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("124"))   // Red

	// Messages
	initialMsg := textStyle.Render(text)
	doneMsg := successStyle.Render("✔ Completed" + strings.Repeat("\t", 50))
	failMsg := failStyle.Render("✖ Failed" + strings.Repeat("\t", 50))

	// Print initial message
	fmt.Print(initialMsg)

	// Start spinner
	done := make(chan error)
	go func() { done <- fn() }()

	for i := 0; ; i++ {
		select {
		case err := <-done: // Function completed
			if err != nil {
				fmt.Println("\r" + failMsg)
				return err
			}
			fmt.Println("\r" + doneMsg)
			return nil
		default:
			// Update spinner
			frame := spinnerStyle.Render(frames[i%len(frames)])
			fmt.Printf("\r%s %s", frame, initialMsg)
			time.Sleep(150 * time.Millisecond)
		}
	}
}

func loading(initialMsg, doneMsg, failMsg string, fn func() error) error {
	done := make(chan struct{})
	errc := make(chan error)
	go func() {
		defer close(done)

		s := spinner.New(spinner.CharSets[11], 100*time.Millisecond, spinner.WithWriter(iostream.Messages))
		s.Prefix = initialMsg
		s.FinalMSG = doneMsg
		s.HideCursor = true
		s.Writer = iostream.Messages

		if err := s.Color(spinnerColor); err != nil {
			panic(auth0.Error(err, "failed setting spinner color"))
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
