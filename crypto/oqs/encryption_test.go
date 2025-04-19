package oqs

import (
	"testing"
)

// Test function for GenerateBytesFromParams and GenerateParamsEncryptionSchemesFromBytes

func TestGenerateBytesAndParseFunctions(t *testing.T) {
	// Define the encryption parameters to test
	sigName := "ExampleSigName"
	pubKeyLength := 2048
	privateKeyLength := 1024
	signatureLength := 512
	isPaused := false

	// Generate byte slice from parameters
	byteSlice, err := GenerateBytesFromParams(sigName, pubKeyLength, privateKeyLength, signatureLength, isPaused)
	if err != nil {
		t.Fatalf("GenerateBytesFromParams failed: %v", err)
	}

	if len(byteSlice) != totalLength {
		t.Fatalf("unexpected byte slice length: got %d, want %d", len(byteSlice), totalLength)
	}

	// Use the byte slice to parse back into parameters
	parsedSigName, parsedPubKeyLength, parsedPrivateKeyLength, parsedSignatureLength, parsedIsPaused, err := GenerateParamsEncryptionSchemesFromBytes(byteSlice)
	if err != nil {
		t.Fatalf("GenerateParamsEncryptionSchemesFromBytes failed: %v", err)
	}

	// Assert that the parsed values match the original values
	if parsedSigName != sigName {
		t.Errorf("SigName mismatch: got %s, want %s", parsedSigName, sigName)
	}
	if parsedPubKeyLength != pubKeyLength {
		t.Errorf("PubKeyLength mismatch: got %d, want %d", parsedPubKeyLength, pubKeyLength)
	}
	if parsedPrivateKeyLength != privateKeyLength {
		t.Errorf("PrivateKeyLength mismatch: got %d, want %d", parsedPrivateKeyLength, privateKeyLength)
	}
	if parsedSignatureLength != signatureLength {
		t.Errorf("SignatureLength mismatch: got %d, want %d", parsedSignatureLength, signatureLength)
	}
	if parsedIsPaused != isPaused {
		t.Errorf("IsPaused mismatch: got %t, want %t", parsedIsPaused, isPaused)
	}
}
