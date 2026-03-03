// ABOUTME: DAVE MLS session manager - handles the full E2E key handshake.
// ABOUTME: Mirrors the logic in Discord's TypeScript DaveSessionManager sample.

package dave

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/voice/voicegateway"
)

const initTransitionID = 0
const newGroupEpoch = "1"

// Session manages a DAVE E2E encryption session for one voice channel.
// It maintains the MLS handshake state and per-user encryptors/decryptors.
type Session struct {
	mls        *MlsSession
	encryptor  *Encryptor
	decryptors map[discord.UserID]*Decryptor
	ratchets   map[discord.UserID]*KeyRatchet

	// ssrcToUser maps RTP SSRC numbers to Discord user IDs.
	ssrcToUser map[uint32]discord.UserID

	// transitions stores pending (transitionID → protocolVersion) pairs.
	transitions map[uint32]uint16

	// latestVersion is the most recently prepared protocol version.
	latestVersion uint16

	selfUserID discord.UserID
	groupID    uint64
	recognized map[discord.UserID]struct{}

	gw *voicegateway.Gateway
	mu sync.Mutex

	// externalSenderSet is true once SetExternalSender has been called.
	externalSenderSet bool
}

// NewSession creates a new DAVE session.
func NewSession(gw *voicegateway.Gateway, selfUserID discord.UserID, groupID uint64) (*Session, error) {
	mls, err := NewMlsSession("")
	if err != nil {
		return nil, fmt.Errorf("dave: create MLS session: %w", err)
	}

	enc, err := NewEncryptor()
	if err != nil {
		mls.Close()
		return nil, fmt.Errorf("dave: create encryptor: %w", err)
	}

	return &Session{
		mls:         mls,
		encryptor:   enc,
		decryptors:  make(map[discord.UserID]*Decryptor),
		ratchets:    make(map[discord.UserID]*KeyRatchet),
		ssrcToUser:  make(map[uint32]discord.UserID),
		transitions: make(map[uint32]uint16),
		recognized:  make(map[discord.UserID]struct{}),
		selfUserID:  selfUserID,
		groupID:     groupID,
		gw:          gw,
	}, nil
}

// Close releases all resources held by the session.
func (s *Session) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.mls.Close()
	s.encryptor.Close()
	for _, d := range s.decryptors {
		d.Close()
	}
	for _, r := range s.ratchets {
		r.Close()
	}
}

// AddUser marks a user as a recognised group member.
func (s *Session) AddUser(userID discord.UserID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recognized[userID] = struct{}{}
	s.setupDecryptorForUser(userID)
}

// RemoveUser removes a user from the session.
func (s *Session) RemoveUser(userID discord.UserID) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.recognized, userID)
	if d, ok := s.decryptors[userID]; ok {
		d.Close()
		delete(s.decryptors, userID)
	}
}

// SetUserSSRC registers the mapping from an RTP SSRC to a Discord user ID.
// This is required so Decrypt can find the right decryptor per packet.
func (s *Session) SetUserSSRC(userID discord.UserID, ssrc uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ssrcToUser[ssrc] = userID
	s.setupDecryptorForUser(userID)
}

// OnPrepareEpoch handles DavePrepareEpochEvent (Op 24).
func (s *Session) OnPrepareEpoch(epoch string, version uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if epoch == newGroupEpoch {
		s.mls.Init(version, s.groupID, s.selfUserID.String())
	}

	if epoch == newGroupEpoch {
		return s.sendKeyPackage()
	}
	return nil
}

// OnExternalSenderPackage handles MLSExternalSenderPackageEvent (Op 25).
func (s *Session) OnExternalSenderPackage(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.mls.SetExternalSender(data)
	s.externalSenderSet = true
}

// OnProposals handles MLSProposalsEvent (Op 27).
func (s *Session) OnProposals(ctx context.Context, proposals []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	commitWelcome := s.mls.ProcessProposals(proposals, s.recognizedUserIDStrings())
	if len(commitWelcome) == 0 {
		return nil
	}
	return s.gw.Send(ctx, &voicegateway.MLSCommitWelcomeCommand{CommitWelcome: commitWelcome})
}

// OnPrepareCommitTransition handles MLSPrepareCommitTransitionEvent (Op 29).
func (s *Session) OnPrepareCommitTransition(ctx context.Context, transitionID uint32, commit []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := s.mls.ProcessCommit(commit)
	defer result.Close()

	if result.IsIgnored() {
		return nil
	}
	if result.IsFailed() {
		s.sendInvalidCommitWelcome(ctx, transitionID)
		s.mls.Reset()
		return s.sendKeyPackage()
	}

	// Joined the group via this commit.
	rosterIDs := result.RosterMemberIDs()
	if len(rosterIDs) > 0 {
		s.prepareRatchets(transitionID, s.mls.GetProtocolVersion())
		return s.maybeSendReadyForTransition(ctx, transitionID)
	}

	s.sendInvalidCommitWelcome(ctx, transitionID)
	s.mls.Reset()
	return s.sendKeyPackage()
}

// OnWelcome handles MLSWelcomeEvent (Op 30).
func (s *Session) OnWelcome(ctx context.Context, transitionID uint32, welcome []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	rosterIDs := s.mls.ProcessWelcome(welcome, s.recognizedUserIDStrings())
	if rosterIDs != nil {
		s.prepareRatchets(transitionID, s.mls.GetProtocolVersion())
		return s.maybeSendReadyForTransition(ctx, transitionID)
	}

	s.sendInvalidCommitWelcome(ctx, transitionID)
	return s.sendKeyPackage()
}

// OnPrepareTransition handles DavePrepareTransitionEvent (Op 21).
func (s *Session) OnPrepareTransition(ctx context.Context, transitionID uint32, version uint16) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prepareRatchets(transitionID, version)
	return s.maybeSendReadyForTransition(ctx, transitionID)
}

// OnExecuteTransition handles DaveExecuteTransitionEvent (Op 22).
func (s *Session) OnExecuteTransition(transitionID uint32) {
	s.mu.Lock()
	defer s.mu.Unlock()

	version, ok := s.transitions[transitionID]
	if !ok {
		return
	}
	delete(s.transitions, transitionID)

	if version == 0 {
		s.mls.Reset()
	}

	// Apply our own encryptor's ratchet.
	s.setupEncryptorRatchet(version)
}

// Encrypt DAVE-encrypts an Opus frame. The SSRC is the bot's own SSRC.
// If no key ratchet is set yet, the frame is returned as-is.
func (s *Session) Encrypt(ssrc uint32, opus []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.encryptor.Encrypt(ssrc, opus)
}

// Decrypt DAVE-decrypts an incoming Opus frame identified by SSRC.
// If no decryptor exists for the SSRC, the frame is returned as-is.
func (s *Session) Decrypt(ssrc uint32, opus []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	userID, ok := s.ssrcToUser[ssrc]
	if !ok {
		return opus, nil
	}
	d, ok := s.decryptors[userID]
	if !ok {
		return opus, nil
	}
	return d.Decrypt(opus)
}

// --- internal helpers (must be called with s.mu held) ---

func (s *Session) sendKeyPackage() error {
	kp := s.mls.MarshalledKeyPackage()
	ctx := context.Background()
	return s.gw.Send(ctx, &voicegateway.MLSKeyPackageCommand{KeyPackage: kp})
}

func (s *Session) maybeSendReadyForTransition(ctx context.Context, transitionID uint32) error {
	if transitionID == initTransitionID {
		return nil
	}
	return s.gw.Send(ctx, &voicegateway.DaveReadyForTransitionCommand{TransitionID: transitionID})
}

func (s *Session) sendInvalidCommitWelcome(ctx context.Context, transitionID uint32) {
	if err := s.gw.Send(ctx, &voicegateway.MLSInvalidCommitWelcomeCommand{TransitionID: transitionID}); err != nil {
		log.Printf("dave: send MLSInvalidCommitWelcome: %v", err)
	}
}

func (s *Session) prepareRatchets(transitionID uint32, version uint16) {
	for userID := range s.recognized {
		if userID == s.selfUserID {
			continue
		}
		s.setupUserRatchet(userID, version)
	}

	if transitionID == initTransitionID {
		s.setupEncryptorRatchet(version)
	} else {
		s.transitions[transitionID] = version
	}
	s.latestVersion = version
}

func (s *Session) setupUserRatchet(userID discord.UserID, version uint16) {
	if r, ok := s.ratchets[userID]; ok {
		r.Close()
	}
	if version == 0 {
		delete(s.ratchets, userID)
		if d, ok := s.decryptors[userID]; ok {
			d.TransitionToKeyRatchet(nil)
		}
		return
	}
	r := s.mls.KeyRatchet(userID.String())
	if r == nil {
		return
	}
	s.ratchets[userID] = r
	s.setupDecryptorForUser(userID)
}

func (s *Session) setupDecryptorForUser(userID discord.UserID) {
	r, hasRatchet := s.ratchets[userID]
	if !hasRatchet {
		return
	}
	d, ok := s.decryptors[userID]
	if !ok {
		var err error
		d, err = NewDecryptor()
		if err != nil {
			log.Printf("dave: create decryptor for %s: %v", userID, err)
			return
		}
		s.decryptors[userID] = d
	}
	d.TransitionToKeyRatchet(r)
}

func (s *Session) setupEncryptorRatchet(version uint16) {
	if r, ok := s.ratchets[s.selfUserID]; ok {
		r.Close()
		delete(s.ratchets, s.selfUserID)
	}
	if version == 0 {
		return
	}
	r := s.mls.KeyRatchet(s.selfUserID.String())
	if r == nil {
		return
	}
	s.ratchets[s.selfUserID] = r
	s.encryptor.SetKeyRatchet(r)
}

func (s *Session) recognizedUserIDStrings() []string {
	out := make([]string, 0, len(s.recognized)+1)
	for uid := range s.recognized {
		out = append(out, uid.String())
	}
	out = append(out, s.selfUserID.String())
	return out
}
