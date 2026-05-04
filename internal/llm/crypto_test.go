package llm

import "testing"

func TestEncryptDecryptAPIKey(t *testing.T) {
	original := "sk-test-api-key-12345"

	encrypted, err := encryptAPIKey(original)
	if err != nil {
		t.Fatalf("encryptAPIKey: %v", err)
	}
	if encrypted == original {
		t.Error("encrypted should differ from original")
	}

	decrypted, err := decryptAPIKey(encrypted)
	if err != nil {
		t.Fatalf("decryptAPIKey: %v", err)
	}
	if decrypted != original {
		t.Errorf("decrypted = %q, want %q", decrypted, original)
	}
}

func TestMaskAPIKey(t *testing.T) {
	tests := []struct {
		key  string
		want string
	}{
		{"sk-proj-abc123xyz789", "sk-***789"},
		{"short", "***"},
		{"abc", "***"},
	}
	for _, tt := range tests {
		got := MaskAPIKey(tt.key)
		if got != tt.want {
			t.Errorf("MaskAPIKey(%q) = %q, want %q", tt.key, got, tt.want)
		}
	}
}
