package voicegateway

import (
	"strconv"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/ws"
)

//go:generate go run ../../utils/cmd/genevent -p voicegateway -o event_methods.go

// OpUnmarshalers contains the Op unmarshalers for the voice gateway events.
var OpUnmarshalers = ws.NewOpUnmarshalers()

// IdentifyCommand is a command for Op 0.
//
// https://discord.com/developers/docs/topics/voice-connections#establishing-a-voice-websocket-connection-example-voice-identify-payload
type IdentifyCommand struct {
	GuildID             discord.GuildID `json:"server_id"` // yes, this should be "server_id"
	UserID              discord.UserID  `json:"user_id"`
	SessionID           string          `json:"session_id"`
	Token               string          `json:"token"`
	MaxDAVEProtocolVersion uint16          `json:"max_dave_protocol_version,omitempty"`
}

// SelectProtocolCommand is a command for Op 1.
//
// https://discord.com/developers/docs/topics/voice-connections#establishing-a-voice-udp-connection-example-select-protocol-payload
type SelectProtocolCommand struct {
	Protocol string             `json:"protocol"`
	Data     SelectProtocolData `json:"data"`
}

// SelectProtocolData is the data inside a SelectProtocolCommand.
type SelectProtocolData struct {
	Address             string `json:"address"`
	Port                uint16 `json:"port"`
	Mode                string `json:"mode"`
	DAVEProtocolVersion uint16 `json:"dave_protocol_version,omitempty"`
}

// ReadyEvent is an event for Op 2.
//
// https://discord.com/developers/docs/topics/voice-connections#establishing-a-voice-websocket-connection-example-voice-ready-payload
type ReadyEvent struct {
	SSRC        uint32   `json:"ssrc"`
	IP          string   `json:"ip"`
	Port        int      `json:"port"`
	Modes       []string `json:"modes"`
	Experiments []string `json:"experiments"`

	// From Discord's API Docs:
	//
	// `heartbeat_interval` here is an erroneous field and should be ignored.
	// The correct `heartbeat_interval` value comes from the Hello payload.

	// HeartbeatInterval discord.Milliseconds `json:"heartbeat_interval"`
}

// Addr formats the URL inside Ready to be of format "host:port".
func (r ReadyEvent) Addr() string {
	return r.IP + ":" + strconv.Itoa(r.Port)
}

// HeartbeatCommand is a command for Op 3.
// Voice gateway v8 requires both a nonce and the last received sequence number.
//
// https://discord.com/developers/docs/topics/voice-connections#heartbeating-example-heartbeat-payload
type HeartbeatCommand struct {
	Nonce  uint64 `json:"t"`
	SeqAck int64  `json:"seq_ack"`
}

// SessionDescriptionEvent is an event for Op 4.
//
// https://discord.com/developers/docs/topics/voice-connections#establishing-a-voice-udp-connection-example-session-description-payload
type SessionDescriptionEvent struct {
	Mode                string   `json:"mode"`
	SecretKey           [32]byte `json:"secret_key"`
	DAVEProtocolVersion uint16   `json:"dave_protocol_version,omitempty"`
}

// https://discord.com/developers/docs/topics/voice-connections#speaking
type SpeakingFlag uint64

const NotSpeaking SpeakingFlag = 0

const (
	Microphone SpeakingFlag = 1 << iota
	Soundshare
	Priority
)

// SpeakingEvent is an event for Op 5. It is also a command.
//
// https://discord.com/developers/docs/topics/voice-connections#speaking-example-speaking-payload
type SpeakingEvent struct {
	Speaking SpeakingFlag   `json:"speaking"`
	Delay    int            `json:"delay"`
	SSRC     uint32         `json:"ssrc"`
	UserID   discord.UserID `json:"user_id,omitempty"`
}

// HeartbeatAckEvent is an event for Op 6.
//
// https://discord.com/developers/docs/topics/voice-connections#heartbeating-example-heartbeat-ack-payload
type HeartbeatAckEvent uint64

// ResumeCommand is a command for Op 7.
// Voice gateway v8 requires seq_ack to indicate the last received sequence number.
//
// https://discord.com/developers/docs/topics/voice-connections#resuming-voice-connection-example-resume-connection-payload
type ResumeCommand struct {
	GuildID   discord.GuildID `json:"server_id"` // yes, this should be "server_id"
	SessionID string          `json:"session_id"`
	Token     string          `json:"token"`
	SeqAck    int64           `json:"seq_ack"`
}

// HelloEvent is an event for Op 8.
//
// https://discord.com/developers/docs/topics/voice-connections#heartbeating-example-hello-payload-since-v3
type HelloEvent struct {
	HeartbeatInterval discord.Milliseconds `json:"heartbeat_interval"`
}

// ResumedEvent is an event for Op 9.
// https://discord.com/developers/docs/topics/voice-connections#resuming-voice-connection-example-resumed-payload
type ResumedEvent struct{}

// ClientsConnectEvent is an event for Op 11. It is sent when one or more
// clients have connected to the voice channel.
type ClientsConnectEvent struct {
	UserIDs []discord.UserID `json:"user_ids"`
}

// ClientDisconnectEvent is an event for Op 13. It is undocumented, but its
// existence is mentioned in this issue:
// https://github.com/discord/discord-api-docs/issues/510.
type ClientDisconnectEvent struct {
	UserID discord.UserID `json:"user_id"`
}

// DavePrepareTransitionEvent is an event for Op 21.
// The server signals an upcoming key rotation. The client should prepare
// new key ratchets for the given transition ID and protocol version.
type DavePrepareTransitionEvent struct {
	TransitionID    uint32 `json:"transition_id"`
	ProtocolVersion uint16 `json:"protocol_version"`
}

// DaveExecuteTransitionEvent is an event for Op 22.
// The server signals that the previously prepared key transition should
// now be applied. The client switches to the new key ratchets.
type DaveExecuteTransitionEvent struct {
	TransitionID uint32 `json:"transition_id"`
}

// DaveReadyForTransitionCommand is a command for Op 23.
// Sent by the client after preparing ratchets for a non-init transition,
// to signal readiness to the server.
type DaveReadyForTransitionCommand struct {
	TransitionID uint32 `json:"transition_id"`
}

// DavePrepareEpochEvent is an event for Op 24.
// The server requests that the client initialise a new MLS group epoch.
// Epoch "1" indicates a fresh group; the client should call Init and send
// its key package.
type DavePrepareEpochEvent struct {
	Epoch           string `json:"epoch"`
	ProtocolVersion uint16 `json:"protocol_version"`
}

// MLSExternalSenderPackageEvent is an event for Op 25.
// Contains the MLS external sender credential bytes that must be passed to
// the session before processing any proposals.
type MLSExternalSenderPackageEvent struct {
	Package []byte `json:"package"`
}

// MLSKeyPackageCommand is a command for Op 26.
// Sent by the client to share its MLS key package with the server, which
// distributes it to other group members.
type MLSKeyPackageCommand struct {
	KeyPackage []byte `json:"key_package"`
}

// BinaryPayload implements BinaryEvent. Op 26 is sent as a binary frame.
func (c *MLSKeyPackageCommand) BinaryPayload() []byte { return c.KeyPackage }

// MLSProposalsEvent is an event for Op 27.
// Contains serialised MLS proposals from other group members. The client
// processes them and sends back a commit+welcome.
type MLSProposalsEvent struct {
	Proposals []byte `json:"proposals"`
}

// MLSCommitWelcomeCommand is a command for Op 28.
// Contains the serialised MLS commit and welcome bytes produced after
// processing proposals.
type MLSCommitWelcomeCommand struct {
	CommitWelcome []byte `json:"commit_welcome"`
}

// BinaryPayload implements BinaryEvent. Op 28 is sent as a binary frame.
func (c *MLSCommitWelcomeCommand) BinaryPayload() []byte { return c.CommitWelcome }

// MLSPrepareCommitTransitionEvent is an event for Op 29.
// Contains an MLS commit that the client should process. If the client
// joined the group via this commit, it prepares new ratchets.
type MLSPrepareCommitTransitionEvent struct {
	TransitionID uint32 `json:"transition_id"`
	Commit       []byte `json:"commit"`
}

// MLSWelcomeEvent is an event for Op 30.
// Contains an MLS welcome that the client should process to join the group.
// If successful, the client prepares new ratchets.
type MLSWelcomeEvent struct {
	TransitionID uint32 `json:"transition_id"`
	Welcome      []byte `json:"welcome"`
}

// MLSInvalidCommitWelcomeCommand is a command for Op 31.
// Sent by the client when it cannot process a commit or welcome, signalling
// the server to restart the MLS handshake.
type MLSInvalidCommitWelcomeCommand struct {
	TransitionID uint32 `json:"transition_id"`
}
