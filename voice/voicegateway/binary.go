// ABOUTME: Binary WebSocket frame encoding/decoding for DAVE MLS protocol messages.
// ABOUTME: Discord sends opcodes 25, 27, 29, 30 as binary frames: [2-byte seq][1-byte op][payload].

package voicegateway

import (
	"context"
	"encoding/binary"
	"log"

	"github.com/diamondburned/arikawa/v3/utils/ws"
)

// BinaryEvent marks a voice gateway command that must be sent as a binary
// WebSocket frame instead of a JSON text frame.
type BinaryEvent interface {
	ws.Event
	BinaryPayload() []byte
}

// encodeBinaryFrame encodes a client-sent binary command as [1-byte op][payload].
// Client-to-server binary frames omit the sequence number.
func encodeBinaryFrame(ev BinaryEvent) []byte {
	payload := ev.BinaryPayload()
	frame := make([]byte, 1+len(payload))
	frame[0] = byte(ev.Op())
	copy(frame[1:], payload)
	return frame
}

// decodeBinaryVoiceFrame decodes a server-sent binary frame.
// Format: [2-byte seq BE][1-byte op][payload]
// Opcodes 25, 27, 29, 30 are sent as binary by Discord.
func decodeBinaryVoiceFrame(ctx context.Context, data []byte, out chan<- ws.Op) error {
	if len(data) < 3 {
		return nil
	}
	// data[0:2] = sequence number (ignored for dispatch)
	opcode := ws.OpCode(data[2])
	payload := data[3:]

	var event ws.Event
	switch opcode {
	case 25: // DAVE MLS External Sender Package
		event = &MLSExternalSenderPackageEvent{Package: payload}
	case 27: // DAVE MLS Proposals
		event = &MLSProposalsEvent{Proposals: payload}
	case 29: // DAVE MLS Announce Commit Transition: [2-byte transition_id][commit bytes]
		if len(payload) < 2 {
			return nil
		}
		transitionID := uint32(binary.BigEndian.Uint16(payload[:2]))
		event = &MLSPrepareCommitTransitionEvent{TransitionID: transitionID, Commit: payload[2:]}
	case 30: // DAVE MLS Welcome: [2-byte transition_id][welcome bytes]
		if len(payload) < 2 {
			return nil
		}
		transitionID := uint32(binary.BigEndian.Uint16(payload[:2]))
		event = &MLSWelcomeEvent{TransitionID: transitionID, Welcome: payload[2:]}
	default:
		log.Printf("DAVE: unknown binary opcode %d (%d bytes)", opcode, len(data))
		return nil
	}

	select {
	case out <- ws.Op{Code: opcode, Data: event}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
