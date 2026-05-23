package unit

import (
	"fmt"
	"strings"
	"testing"
)

func TestValidatePamU2FCredential(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			"valid credential",
			"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop,DEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrs,es256,+presence",
			false,
		},
		{
			"valid minimal (just id and key)",
			"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop,DEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrs",
			false,
		},
		{
			"empty string",
			"",
			true,
		},
		{
			"no commas",
			"justonestring",
			true,
		},
		{
			"credential_id too short",
			"short,DEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrs,es256",
			true,
		},
		{
			"public_key too short",
			"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop,short,es256",
			true,
		},
		{
			"whitespace trimmed",
			"  ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnop,DEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrs,es256  ",
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePamU2FCredential(tt.input)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

// Mirror of the validation function from internal/web/api/fido2.go
func validatePamU2FCredential(data string) error {
	data = strings.TrimSpace(data)
	if data == "" {
		return fmt.Errorf("credential_data is empty")
	}

	parts := strings.Split(data, ",")
	if len(parts) < 2 {
		return fmt.Errorf("credential_data format invalid: expected at least credential_id,public_key separated by commas")
	}

	credID := parts[0]
	if len(credID) < 10 {
		return fmt.Errorf("credential_id too short (expected base64url-encoded value)")
	}

	pubKey := parts[1]
	if len(pubKey) < 10 {
		return fmt.Errorf("public_key too short (expected base64url-encoded value)")
	}

	return nil
}
