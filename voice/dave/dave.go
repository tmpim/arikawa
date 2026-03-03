// ABOUTME: CGo wrapper around libdave's C API (dave.h).
// ABOUTME: Provides Go types for DAVE session, encryptor, decryptor, and key ratchet.

package dave

/*
#cgo CFLAGS: -I${SRCDIR}/libdave/cpp/build/install/include
#include "dave/dave.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	"unsafe"
)

// MaxSupportedProtocolVersion returns the highest DAVE protocol version this
// build of libdave supports.
func MaxSupportedProtocolVersion() uint16 {
	return uint16(C.daveMaxSupportedProtocolVersion())
}

// MlsSession wraps a DAVESessionHandle.
type MlsSession struct {
	h C.DAVESessionHandle
}

// NewMlsSession creates a new DAVE MLS session.
// authSessionID can be empty; it is only used for persistent key storage.
func NewMlsSession(authSessionID string) (*MlsSession, error) {
	cAuth := C.CString(authSessionID)
	defer C.free(unsafe.Pointer(cAuth))

	h := C.daveSessionCreate(nil, cAuth, nil, nil)
	if h == nil {
		return nil, fmt.Errorf("daveSessionCreate returned nil")
	}
	return &MlsSession{h: h}, nil
}

// Close frees the underlying C session.
func (s *MlsSession) Close() {
	if s.h != nil {
		C.daveSessionDestroy(s.h)
		s.h = nil
	}
}

// Init initialises the session for the given protocol version, group, and user.
func (s *MlsSession) Init(version uint16, groupID uint64, selfUserID string) {
	cUser := C.CString(selfUserID)
	defer C.free(unsafe.Pointer(cUser))
	C.daveSessionInit(s.h, C.uint16_t(version), C.uint64_t(groupID), cUser)
}

// Reset resets the session state.
func (s *MlsSession) Reset() {
	C.daveSessionReset(s.h)
}

// SetProtocolVersion updates the session's protocol version.
func (s *MlsSession) SetProtocolVersion(version uint16) {
	C.daveSessionSetProtocolVersion(s.h, C.uint16_t(version))
}

// GetProtocolVersion returns the session's current protocol version.
func (s *MlsSession) GetProtocolVersion() uint16 {
	return uint16(C.daveSessionGetProtocolVersion(s.h))
}

// SetExternalSender sets the external sender credential bytes.
func (s *MlsSession) SetExternalSender(data []byte) {
	if len(data) == 0 {
		return
	}
	C.daveSessionSetExternalSender(s.h,
		(*C.uint8_t)(unsafe.Pointer(&data[0])),
		C.size_t(len(data)))
}

// ProcessProposals processes incoming MLS proposals and returns the
// commit+welcome bytes to send, or nil if nothing should be sent.
func (s *MlsSession) ProcessProposals(proposals []byte, recognizedUserIDs []string) []byte {
	if len(proposals) == 0 {
		return nil
	}

	cUsers, freeUsers := toCStringArray(recognizedUserIDs)
	defer freeUsers()

	var outPtr *C.uint8_t
	var outLen C.size_t

	C.daveSessionProcessProposals(s.h,
		(*C.uint8_t)(unsafe.Pointer(&proposals[0])),
		C.size_t(len(proposals)),
		cUsers, C.size_t(len(recognizedUserIDs)),
		&outPtr, &outLen)

	return cBytesToGo(outPtr, outLen)
}

// CommitResult wraps the result of processing an MLS commit.
type CommitResult struct {
	h C.DAVECommitResultHandle
}

// IsFailed reports whether processing the commit failed.
func (r *CommitResult) IsFailed() bool {
	if r == nil || r.h == nil {
		return true
	}
	return bool(C.daveCommitResultIsFailed(r.h))
}

// IsIgnored reports whether the commit should be ignored.
func (r *CommitResult) IsIgnored() bool {
	if r == nil || r.h == nil {
		return false
	}
	return bool(C.daveCommitResultIsIgnored(r.h))
}

// RosterMemberIDs returns the list of user IDs in the roster after the commit.
func (r *CommitResult) RosterMemberIDs() []uint64 {
	if r == nil || r.h == nil {
		return nil
	}
	var ptr *C.uint64_t
	var length C.size_t
	C.daveCommitResultGetRosterMemberIds(r.h, &ptr, &length)
	if ptr == nil || length == 0 {
		return nil
	}
	defer C.daveFree(unsafe.Pointer(ptr))
	out := make([]uint64, int(length))
	slice := (*[1 << 28]C.uint64_t)(unsafe.Pointer(ptr))[:int(length):int(length)]
	for i, v := range slice {
		out[i] = uint64(v)
	}
	return out
}

// Close frees the commit result.
func (r *CommitResult) Close() {
	if r == nil || r.h == nil {
		return
	}
	C.daveCommitResultDestroy(r.h)
	r.h = nil
}

// ProcessCommit processes an incoming MLS commit and returns a CommitResult.
// Caller must call Close() on the result.
func (s *MlsSession) ProcessCommit(commit []byte) *CommitResult {
	if len(commit) == 0 {
		return nil
	}
	h := C.daveSessionProcessCommit(s.h,
		(*C.uint8_t)(unsafe.Pointer(&commit[0])),
		C.size_t(len(commit)))
	if h == nil {
		return nil
	}
	return &CommitResult{h: h}
}

// ProcessWelcome processes an MLS welcome and returns the roster member IDs,
// or nil if joining failed.
func (s *MlsSession) ProcessWelcome(welcome []byte, recognizedUserIDs []string) []uint64 {
	if len(welcome) == 0 {
		return nil
	}

	cUsers, freeUsers := toCStringArray(recognizedUserIDs)
	defer freeUsers()

	h := C.daveSessionProcessWelcome(s.h,
		(*C.uint8_t)(unsafe.Pointer(&welcome[0])),
		C.size_t(len(welcome)),
		cUsers, C.size_t(len(recognizedUserIDs)))

	if h == nil {
		return nil
	}
	defer C.daveWelcomeResultDestroy(h)

	var ptr *C.uint64_t
	var length C.size_t
	C.daveWelcomeResultGetRosterMemberIds(h, &ptr, &length)
	if ptr == nil || length == 0 {
		return []uint64{}
	}
	defer C.daveFree(unsafe.Pointer(ptr))

	out := make([]uint64, int(length))
	slice := (*[1 << 28]C.uint64_t)(unsafe.Pointer(ptr))[:int(length):int(length)]
	for i, v := range slice {
		out[i] = uint64(v)
	}
	return out
}

// MarshalledKeyPackage returns the MLS key package bytes for this session.
func (s *MlsSession) MarshalledKeyPackage() []byte {
	var outPtr *C.uint8_t
	var outLen C.size_t
	C.daveSessionGetMarshalledKeyPackage(s.h, &outPtr, &outLen)
	return cBytesToGo(outPtr, outLen)
}

// KeyRatchet returns the key ratchet for the given user ID.
// Caller must call Close() on the result.
func (s *MlsSession) KeyRatchet(userID string) *KeyRatchet {
	cUser := C.CString(userID)
	defer C.free(unsafe.Pointer(cUser))
	h := C.daveSessionGetKeyRatchet(s.h, cUser)
	if h == nil {
		return nil
	}
	return &KeyRatchet{h: h}
}

// KeyRatchet wraps a DAVEKeyRatchetHandle.
type KeyRatchet struct {
	h C.DAVEKeyRatchetHandle
}

// Close frees the key ratchet.
func (r *KeyRatchet) Close() {
	if r.h != nil {
		C.daveKeyRatchetDestroy(r.h)
		r.h = nil
	}
}

// Encryptor wraps a DAVEEncryptorHandle.
type Encryptor struct {
	h C.DAVEEncryptorHandle
}

// NewEncryptor creates a new DAVE media frame encryptor.
func NewEncryptor() (*Encryptor, error) {
	h := C.daveEncryptorCreate()
	if h == nil {
		return nil, fmt.Errorf("daveEncryptorCreate returned nil")
	}
	return &Encryptor{h: h}, nil
}

// Close frees the encryptor.
func (e *Encryptor) Close() {
	if e.h != nil {
		C.daveEncryptorDestroy(e.h)
		e.h = nil
	}
}

// SetKeyRatchet sets the key ratchet to use for encryption.
// The encryptor does NOT take ownership; the caller must keep the ratchet alive.
func (e *Encryptor) SetKeyRatchet(r *KeyRatchet) {
	C.daveEncryptorSetKeyRatchet(e.h, r.h)
}

// AssignSsrcToCodec registers an SSRC as carrying Opus audio.
func (e *Encryptor) AssignSsrcToCodec(ssrc uint32) {
	C.daveEncryptorAssignSsrcToCodec(e.h, C.uint32_t(ssrc), C.DAVE_CODEC_OPUS)
}

// Encrypt encrypts an Opus audio frame.
// Returns the original frame unmodified if no key ratchet is set (passthrough).
func (e *Encryptor) Encrypt(ssrc uint32, frame []byte) ([]byte, error) {
	if len(frame) == 0 {
		return frame, nil
	}

	maxSize := C.daveEncryptorGetMaxCiphertextByteSize(e.h,
		C.DAVE_MEDIA_TYPE_AUDIO, C.size_t(len(frame)))

	if maxSize == 0 {
		return frame, nil
	}
	out := make([]byte, int(maxSize))
	var written C.size_t

	rc := C.daveEncryptorEncrypt(e.h,
		C.DAVE_MEDIA_TYPE_AUDIO,
		C.uint32_t(ssrc),
		(*C.uint8_t)(unsafe.Pointer(&frame[0])),
		C.size_t(len(frame)),
		(*C.uint8_t)(unsafe.Pointer(&out[0])),
		maxSize,
		&written)

	switch rc {
	case C.DAVE_ENCRYPTOR_RESULT_CODE_SUCCESS:
		return out[:int(written)], nil
	case C.DAVE_ENCRYPTOR_RESULT_CODE_MISSING_KEY_RATCHET:
		return frame, nil
	default:
		return nil, fmt.Errorf("DAVE encrypt failed: code %d", int(rc))
	}
}

// Decryptor wraps a DAVEDecryptorHandle.
type Decryptor struct {
	h C.DAVEDecryptorHandle
}

// NewDecryptor creates a new DAVE media frame decryptor.
func NewDecryptor() (*Decryptor, error) {
	h := C.daveDecryptorCreate()
	if h == nil {
		return nil, fmt.Errorf("daveDecryptorCreate returned nil")
	}
	return &Decryptor{h: h}, nil
}

// Close frees the decryptor.
func (d *Decryptor) Close() {
	if d.h != nil {
		C.daveDecryptorDestroy(d.h)
		d.h = nil
	}
}

// TransitionToKeyRatchet updates the decryptor to use a new key ratchet.
// The decryptor does NOT take ownership; the caller must keep the ratchet alive.
func (d *Decryptor) TransitionToKeyRatchet(r *KeyRatchet) {
	C.daveDecryptorTransitionToKeyRatchet(d.h, r.h)
}

// SetPassthroughMode enables or disables passthrough mode on the decryptor.
// In passthrough mode, frames are returned unmodified without decryption.
func (d *Decryptor) SetPassthroughMode(passthrough bool) {
	C.daveDecryptorTransitionToPassthroughMode(d.h, C.bool(passthrough))
}

// Decrypt decrypts a DAVE-encrypted Opus audio frame.
// Returns the original frame unmodified if no key ratchet is set.
func (d *Decryptor) Decrypt(frame []byte) ([]byte, error) {
	if len(frame) == 0 {
		return frame, nil
	}

	maxSize := C.daveDecryptorGetMaxPlaintextByteSize(d.h,
		C.DAVE_MEDIA_TYPE_AUDIO, C.size_t(len(frame)))

	if maxSize == 0 {
		return frame, nil
	}
	out := make([]byte, int(maxSize))
	var written C.size_t

	rc := C.daveDecryptorDecrypt(d.h,
		C.DAVE_MEDIA_TYPE_AUDIO,
		(*C.uint8_t)(unsafe.Pointer(&frame[0])),
		C.size_t(len(frame)),
		(*C.uint8_t)(unsafe.Pointer(&out[0])),
		maxSize,
		&written)

	switch rc {
	case C.DAVE_DECRYPTOR_RESULT_CODE_SUCCESS:
		return out[:int(written)], nil
	case C.DAVE_DECRYPTOR_RESULT_CODE_MISSING_KEY_RATCHET:
		return frame, nil
	default:
		return nil, fmt.Errorf("DAVE decrypt failed: code %d", int(rc))
	}
}

// --- helpers ---

// cBytesToGo copies a C byte array into a Go slice and frees the C memory.
func cBytesToGo(ptr *C.uint8_t, length C.size_t) []byte {
	if ptr == nil || length == 0 {
		return nil
	}
	defer C.daveFree(unsafe.Pointer(ptr))
	out := make([]byte, int(length))
	copy(out, (*[1 << 28]byte)(unsafe.Pointer(ptr))[:int(length):int(length)])
	return out
}

// toCStringArray converts a Go string slice to a **C.char array.
// The pointer array is allocated on the C heap to satisfy CGo pointer rules.
// The returned free function must be called when done.
func toCStringArray(strs []string) (**C.char, func()) {
	if len(strs) == 0 {
		return nil, func() {}
	}
	ptrs := make([]*C.char, len(strs))
	for i, s := range strs {
		ptrs[i] = C.CString(s)
	}
	// Allocate the pointer array on the C heap to satisfy CGo pointer rules.
	size := C.size_t(len(ptrs)) * C.size_t(unsafe.Sizeof((*C.char)(nil)))
	cArray := (**C.char)(C.malloc(size))
	for i, p := range ptrs {
		*(**C.char)(unsafe.Pointer(uintptr(unsafe.Pointer(cArray)) + uintptr(i)*unsafe.Sizeof((*C.char)(nil)))) = p
	}
	return cArray, func() {
		for _, p := range ptrs {
			C.free(unsafe.Pointer(p))
		}
		C.free(unsafe.Pointer(cArray))
	}
}
