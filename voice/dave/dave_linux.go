// ABOUTME: Linux-specific CGo linker flags for libdave.
// ABOUTME: Links the merged libdave.a (which includes mlspp and OpenSSL).

package dave

/*
#cgo LDFLAGS: ${SRCDIR}/libdave/cpp/build/install/lib/libdave.a -lstdc++ -lm
*/
import "C"
