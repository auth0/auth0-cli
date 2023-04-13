package display

import (
	"bytes"
	"io"
	"sync"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"

	"github.com/auth0/auth0-cli/internal/auth0"
)

func TestStream(t *testing.T) {
	var stdout bytes.Buffer
	mockRender := &Renderer{
		MessageWriter: io.Discard,
		ResultWriter:  &stdout,
	}

	results := []View{
		&logView{
			Log: &management.Log{
				LogID:       auth0.String("354234"),
				Type:        auth0.String("sapi"),
				Description: auth0.String("Update branding settings"),
			},
		},
	}

	t.Run("Stream correctly handles nil channel", func(t *testing.T) {
		mockRender.Stream(results, nil)
		expectedResult := `TYPE                       DESCRIPTION                                               DATE                    CONNECTION              CLIENT                  
API Operation              Update branding settings                                  Jan 01 00:00:00.000     N/A                     N/A    
`
		assert.Equal(t, expectedResult, stdout.String())
		stdout.Reset()
	})

	t.Run("Stream successfully", func(t *testing.T) {
		viewChan := make(chan View)

		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			mockRender.Stream(results, viewChan)
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()
			viewChan <- &logView{
				Log: &management.Log{
					LogID:       auth0.String("354236"),
					Type:        auth0.String("sapi"),
					Description: auth0.String("Update tenant settings"),
				},
			}
			close(viewChan)
		}()

		wg.Wait()

		expectedResult := `TYPE                       DESCRIPTION                                               DATE                    CONNECTION              CLIENT                  
API Operation              Update branding settings                                  Jan 01 00:00:00.000     N/A                     N/A    
API Operation              Update tenant settings                                    Jan 01 00:00:00.000     N/A                     N/A    
`
		assert.Equal(t, expectedResult, stdout.String())
		stdout.Reset()
	})
}
