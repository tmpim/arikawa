// ABOUTME: Tests for the libdave CGo wrapper.
// ABOUTME: Covers object lifecycle and passthrough (no-key-ratchet) behaviour.

package dave

import (
	"testing"
)

func TestMaxSupportedProtocolVersion(t *testing.T) {
	v := MaxSupportedProtocolVersion()
	if v == 0 {
		t.Fatal("expected non-zero max protocol version")
	}
	t.Logf("max supported DAVE protocol version: %d", v)
}

func TestMlsSessionLifecycle(t *testing.T) {
	s, err := NewMlsSession("")
	if err != nil {
		t.Fatalf("NewMlsSession: %v", err)
	}
	s.Close()
	s.Close() // must not panic
}

func TestEncryptorLifecycle(t *testing.T) {
	e, err := NewEncryptor()
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	e.Close()
	e.Close() // must not panic
}

func TestDecryptorLifecycle(t *testing.T) {
	d, err := NewDecryptor()
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	d.Close()
	d.Close() // must not panic
}

func TestEncryptorPassthrough(t *testing.T) {
	e, err := NewEncryptor()
	if err != nil {
		t.Fatalf("NewEncryptor: %v", err)
	}
	defer e.Close()

	frame := []byte{0x01, 0x02, 0x03, 0x04}
	out, err := e.Encrypt(12345, frame)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if string(out) != string(frame) {
		t.Fatalf("expected passthrough: got %v, want %v", out, frame)
	}
}

func TestDecryptorPassthrough(t *testing.T) {
	d, err := NewDecryptor()
	if err != nil {
		t.Fatalf("NewDecryptor: %v", err)
	}
	defer d.Close()

	// Decryptor requires passthrough mode to be explicitly enabled before it
	// will pass frames through without a key ratchet.
	d.SetPassthroughMode(true)

	frame := []byte{0x01, 0x02, 0x03, 0x04}
	out, err := d.Decrypt(frame)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if string(out) != string(frame) {
		t.Fatalf("expected passthrough: got %v, want %v", out, frame)
	}
}

func TestProcessCommitNilInput(t *testing.T) {
	s, err := NewMlsSession("")
	if err != nil {
		t.Fatalf("NewMlsSession: %v", err)
	}
	defer s.Close()

	result := s.ProcessCommit(nil)
	if result != nil {
		t.Fatal("expected nil CommitResult for nil input")
	}
}

func TestProcessCommitEmptyInput(t *testing.T) {
	s, err := NewMlsSession("")
	if err != nil {
		t.Fatalf("NewMlsSession: %v", err)
	}
	defer s.Close()

	result := s.ProcessCommit([]byte{})
	if result != nil {
		t.Fatal("expected nil CommitResult for empty input")
	}
}

func TestCommitResultNilSafety(t *testing.T) {
	var r *CommitResult
	// All of these must not panic on a nil receiver.
	_ = r.IsFailed()
	_ = r.IsIgnored()
	_ = r.RosterMemberIDs()
	r.Close() // must not panic
}
