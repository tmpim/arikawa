// ABOUTME: linux/amd64 CGo linker flags for prebuilt libdave.
// ABOUTME: libdave.a includes mlspp and OpenSSL statically.

//go:build linux && amd64

package dave

/*
#cgo LDFLAGS: ${SRCDIR}/lib/linux_amd64/libdave.a -lstdc++ -lm
*/
import "C"
