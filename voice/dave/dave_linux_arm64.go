// ABOUTME: linux/arm64 CGo linker flags for prebuilt libdave.
// ABOUTME: libdave.a includes mlspp and OpenSSL statically.

//go:build linux && arm64

package dave

/*
#cgo LDFLAGS: ${SRCDIR}/lib/linux_arm64/libdave.a -lm
*/
import "C"
