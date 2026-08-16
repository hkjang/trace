package crypto

import "testing"

func TestVaultRoundTripAndPurposeBinding(t *testing.T) {
	v, err := NewVault(make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	sealed, err := v.Seal([]byte("secret"), "ai.api_key")
	if err != nil {
		t.Fatal(err)
	}
	plain, err := v.Open(sealed, "ai.api_key")
	if err != nil || string(plain) != "secret" {
		t.Fatalf("Open() = %q, %v", plain, err)
	}
	if _, err := v.Open(sealed, "oidc.client_secret"); err == nil {
		t.Fatal("purpose mismatch should fail")
	}
}
