package unit

import (
	"testing"

	"github.com/authbox/authbox/internal/web/api"
)

func TestValidateServiceToken(t *testing.T) {
	t.Run("invalid token", func(t *testing.T) {
		_, _, ok := api.ValidateServiceToken("nonexistent")
		if ok {
			t.Fatal("expected invalid token to fail")
		}
	})

	t.Run("empty token", func(t *testing.T) {
		_, _, ok := api.ValidateServiceToken("")
		if ok {
			t.Fatal("expected empty token to fail")
		}
	})
}
