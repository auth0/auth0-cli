package display

import (
	"fmt"
	"testing"

	"github.com/auth0/go-auth0/management"
	"github.com/stretchr/testify/assert"
)

func TestGetScopes(t *testing.T) {
	t.Run("no scopes should not truncate", func(t *testing.T) {
		mockScopes := []management.ResourceServerScope{}

		getScopes(mockScopes)

		scopes, didTruncate := getScopes(mockScopes)
		assert.Equal(t, "", scopes)
		assert.False(t, didTruncate)
	})

	t.Run("few scopes should not truncate", func(t *testing.T) {
		mockScopes := []management.ResourceServerScope{}

		for i := 0; i < 3; i++ {
			v := fmt.Sprintf("scope%d", i)
			d := fmt.Sprintf("Description for scope%d", i)

			mockScopes = append(mockScopes, management.ResourceServerScope{
				Value:       &v,
				Description: &d,
			})
		}

		scopes, didTruncate := getScopes(mockScopes)
		assert.Equal(t, "scope0 scope1 scope2", scopes)
		assert.False(t, didTruncate)
	})

	t.Run("should truncate", func(t *testing.T) {
		mockScopes := []management.ResourceServerScope{}

		for i := 0; i < 100; i++ {
			v := fmt.Sprintf("scope%d", i)
			d := fmt.Sprintf("Description for scope%d", i)

			mockScopes = append(mockScopes, management.ResourceServerScope{
				Value:       &v,
				Description: &d,
			})
		}

		scopes, didTruncate := getScopes(mockScopes)
		assert.Equal(t, "scope0 scope1 scope2 scope3 scope4 scope5 scope6...", scopes)
		assert.True(t, didTruncate)
	})
}

func TestApiView_AsTableHeader(t *testing.T) {
	mockApiView := apiView{}
	assert.Equal(t, []string{}, mockApiView.AsTableHeader())
}

func TestApiView_AsTableRow(t *testing.T) {
	mockApiView := apiView{}
	assert.Equal(t, []string{}, mockApiView.AsTableRow())
}
