// Copyright (c) 2026 Forke Inc. (https://www.forke.space/)
// Source-Available License (Non-Commercial / Fair Source).
// Open for inspection, learning, and non-commercial development.
// Commercial use, hosting, or resale without authorization is strictly prohibited.

package tokencrypt

import "testing"

func TestDecryptMatchesNodeFormat(t *testing.T) {
	t.Setenv("SANDBOX_TOKEN_KEY", "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	got, err := Decrypt(nodeSample)
	if err != nil || got != "gho_exampletoken123" {
		t.Fatalf("got %q, %v", got, err)
	}
	if plain, _ := Decrypt("legacy"); plain != "legacy" {
		t.Fatal("legacy plaintext should pass through")
	}
}

const nodeSample = "v1:yF+rkOPpcIvNFxcwCerJNxUMw/AaCw1ZClHdEKhMsZ3lakrwFsghjR8jk+hqZvc="
