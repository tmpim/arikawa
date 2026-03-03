// ABOUTME: darwin/arm64 CGo linker flags for prebuilt libdave.
// ABOUTME: libdave.a includes mlspp and OpenSSL statically.

//go:build darwin && arm64

package dave

/*
#cgo LDFLAGS: ${SRCDIR}/lib/darwin_arm64/libdave.a -lc++ -lm
*/
import "C"
