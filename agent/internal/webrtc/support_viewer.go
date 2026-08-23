package webrtc

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/pion/interceptor"
	pionwebrtc "github.com/pion/webrtc/v3"
	"github.com/stangtennis/remote-agent/internal/video"
)

const supportViewerPeerID = "viewer"

func (m *Manager) createSupportViewerPeer(iceServers []pionwebrtc.ICEServer, offerID string, relayOnly bool) error {
	me := &pionwebrtc.MediaEngine{}
	if err := me.RegisterCodec(pionwebrtc.RTPCodecParameters{
		RTPCodecCapability: pionwebrtc.RTPCodecCapability{
			MimeType:    pionwebrtc.MimeTypeH264,
			ClockRate:   90000,
			SDPFmtpLine: "level-asymmetry-allowed=1;packetization-mode=1;profile-level-id=42e01f",
		},
		PayloadType: 96,
	}, pionwebrtc.RTPCodecTypeVideo); err != nil {
		return fmt.Errorf("register viewer H264 codec: %w", err)
	}
	ir := &interceptor.Registry{}
	if err := pionwebrtc.RegisterDefaultInterceptors(me, ir); err != nil {
		return fmt.Errorf("register viewer interceptors: %w", err)
	}
	api := pionwebrtc.NewAPI(pionwebrtc.WithMediaEngine(me), pionwebrtc.WithInterceptorRegistry(ir))
	configuration := pionwebrtc.Configuration{ICEServers: iceServers}
	if relayOnly {
		configuration.ICETransportPolicy = pionwebrtc.ICETransportPolicyRelay
	}
	pc, err := api.NewPeerConnection(configuration)
	if err != nil {
		return fmt.Errorf("create viewer peer: %w", err)
	}
	track, err := video.NewTrack()
	if err != nil {
		_ = pc.Close()
		return err
	}
	sender, err := pc.AddTrack(track.GetTrack())
	if err != nil {
		_ = pc.Close()
		return fmt.Errorf("add viewer video track: %w", err)
	}
	go func() {
		for {
			if _, _, err := sender.ReadRTCP(); err != nil {
				return
			}
		}
	}()

	m.supportViewerMu.Lock()
	oldPC := m.supportViewerPC
	oldTrack := m.supportViewerTrack
	m.supportViewerPC = pc
	m.supportViewerTrack = track
	m.supportViewerOfferID = offerID
	m.supportViewerAnswerSent = false
	m.supportViewerPendingCandidates = nil
	if m.supportViewerPendingRemote == nil {
		m.supportViewerPendingRemote = make(map[string][]pionwebrtc.ICECandidateInit)
	}
	m.supportViewerMu.Unlock()
	if oldPC != nil {
		_ = oldPC.Close()
	}
	if oldTrack != nil {
		oldTrack.Stop()
	}

	track.Start()
	pc.OnICECandidate(func(candidate *pionwebrtc.ICECandidate) {
		if candidate == nil {
			return
		}
		m.supportViewerMu.Lock()
		if m.supportViewerPC != pc {
			m.supportViewerMu.Unlock()
			return
		}
		if !m.supportViewerAnswerSent {
			m.supportViewerPendingCandidates = append(m.supportViewerPendingCandidates, candidate)
			m.supportViewerMu.Unlock()
			return
		}
		m.supportViewerMu.Unlock()
		m.sendSupportViewerICE(candidate, offerID)
	})
	pc.OnConnectionStateChange(func(state pionwebrtc.PeerConnectionState) {
		log.Printf("🔄 Support viewer connection state: %s", state.String())
		if state == pionwebrtc.PeerConnectionStateDisconnected ||
			state == pionwebrtc.PeerConnectionStateFailed ||
			state == pionwebrtc.PeerConnectionStateClosed {
			m.supportViewerMu.RLock()
			isCurrent := m.supportViewerPC == pc
			m.supportViewerMu.RUnlock()
			if isCurrent {
				go m.closeSupportViewerPeer("viewer " + state.String())
			}
		}
	})

	return nil
}

func (m *Manager) handleSupportViewerOffer(signal SignalMessage, iceServers []pionwebrtc.ICEServer) error {
	var payload struct {
		Type    string `json:"type"`
		SDP     string `json:"sdp"`
		PeerID  string `json:"peer_id"`
		OfferID string `json:"offer_id"`
	}
	if err := json.Unmarshal(signal.Payload, &payload); err != nil {
		return err
	}
	if payload.PeerID != supportViewerPeerID || payload.OfferID == "" || payload.SDP == "" {
		return fmt.Errorf("invalid support viewer offer routing")
	}
	if m.videoEncoder == nil {
		return fmt.Errorf("H264 support viewer is unavailable")
	}
	switch m.videoEncoder.GetEncoderName() {
	case "openh264", "nvenc", "qsv", "amf", "h264_nvenc", "h264_qsv", "h264_amf", "videotoolbox":
	default:
		return fmt.Errorf("support viewer encoder is not H264: %s", m.videoEncoder.GetEncoderName())
	}
	if err := m.createSupportViewerPeer(iceServers, payload.OfferID, m.supportIsActive()); err != nil {
		return err
	}
	m.supportViewerMu.RLock()
	pc := m.supportViewerPC
	m.supportViewerMu.RUnlock()
	if err := pc.SetRemoteDescription(pionwebrtc.SessionDescription{Type: pionwebrtc.SDPTypeOffer, SDP: payload.SDP}); err != nil {
		m.closeSupportViewerPeer("invalid viewer offer")
		return err
	}
	m.flushSupportViewerRemoteICE(pc, payload.OfferID)
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		m.closeSupportViewerPeer("viewer answer failed")
		return err
	}
	if err := pc.SetLocalDescription(answer); err != nil {
		m.closeSupportViewerPeer("viewer local description failed")
		return err
	}
	if err := m.writeViewerSignal("answer", map[string]interface{}{
		"type":     "answer",
		"sdp":      answer.SDP,
		"peer_id":  supportViewerPeerID,
		"offer_id": payload.OfferID,
	}); err != nil {
		m.closeSupportViewerPeer("viewer answer signaling failed")
		return err
	}

	m.supportViewerMu.Lock()
	if m.supportViewerPC != pc {
		m.supportViewerMu.Unlock()
		return nil
	}
	m.supportViewerAnswerSent = true
	pending := append([]*pionwebrtc.ICECandidate(nil), m.supportViewerPendingCandidates...)
	m.supportViewerPendingCandidates = nil
	m.supportViewerMu.Unlock()
	for _, candidate := range pending {
		m.sendSupportViewerICE(candidate, payload.OfferID)
	}
	return nil
}

func (m *Manager) flushSupportViewerRemoteICE(pc *pionwebrtc.PeerConnection, offerID string) {
	m.supportViewerMu.Lock()
	pending := append([]pionwebrtc.ICECandidateInit(nil), m.supportViewerPendingRemote[offerID]...)
	delete(m.supportViewerPendingRemote, offerID)
	m.supportViewerMu.Unlock()
	for _, candidate := range pending {
		if err := pc.AddICECandidate(candidate); err != nil {
			log.Printf("⚠️ Support viewer ICE candidate failed: %v", err)
		}
	}
}

func (m *Manager) handleSupportViewerICE(signal SignalMessage) {
	var payload ICEPayload
	if json.Unmarshal(signal.Payload, &payload) != nil || payload.Candidate == "" ||
		payload.PeerID != supportViewerPeerID || payload.OfferID == "" {
		return
	}
	init := pionwebrtc.ICECandidateInit{Candidate: payload.Candidate, SDPMid: &payload.SDPMid}
	if payload.SDPMLineIndex != nil {
		index := uint16(*payload.SDPMLineIndex)
		init.SDPMLineIndex = &index
	}

	m.supportViewerMu.Lock()
	pc := m.supportViewerPC
	if pc == nil || pc.RemoteDescription() == nil || m.supportViewerOfferID != payload.OfferID {
		if m.supportViewerPendingRemote == nil {
			m.supportViewerPendingRemote = make(map[string][]pionwebrtc.ICECandidateInit)
		}
		m.supportViewerPendingRemote[payload.OfferID] = append(m.supportViewerPendingRemote[payload.OfferID], init)
		m.supportViewerMu.Unlock()
		return
	}
	m.supportViewerMu.Unlock()
	if err := pc.AddICECandidate(init); err != nil {
		log.Printf("⚠️ Support viewer remote ICE failed: %v", err)
	}
}

func (m *Manager) sendSupportViewerICE(candidate *pionwebrtc.ICECandidate, offerID string) {
	init := candidate.ToJSON()
	sdpMid := "0"
	if init.SDPMid != nil && *init.SDPMid != "" {
		sdpMid = *init.SDPMid
	}
	var sdpMLineIndex uint16
	if init.SDPMLineIndex != nil {
		sdpMLineIndex = *init.SDPMLineIndex
	}
	if err := m.writeViewerSignal("ice", map[string]interface{}{
		"candidate":     init.Candidate,
		"sdpMid":        sdpMid,
		"sdpMLineIndex": sdpMLineIndex,
		"peer_id":       supportViewerPeerID,
		"offer_id":      offerID,
	}); err != nil {
		log.Printf("❌ Failed to send support viewer ICE candidate: %v", err)
	}
}

func (m *Manager) supportViewerPresent() bool {
	m.supportViewerMu.RLock()
	defer m.supportViewerMu.RUnlock()
	return m.supportViewerPC != nil
}

func (m *Manager) writeSupportViewerFrame(data []byte, duration time.Duration) error {
	m.supportViewerMu.RLock()
	track := m.supportViewerTrack
	active := m.supportViewerPC != nil
	m.supportViewerMu.RUnlock()
	if !active || track == nil {
		return nil
	}
	return track.WriteFrame(data, duration)
}

func (m *Manager) closeSupportViewerPeer(reason string) {
	m.supportViewerMu.Lock()
	pc := m.supportViewerPC
	track := m.supportViewerTrack
	m.supportViewerPC = nil
	m.supportViewerTrack = nil
	m.supportViewerOfferID = ""
	m.supportViewerAnswerSent = false
	m.supportViewerPendingCandidates = nil
	m.supportViewerPendingRemote = nil
	m.supportViewerMu.Unlock()
	if track != nil {
		track.Stop()
	}
	if pc != nil {
		log.Printf("🧹 Closing support viewer peer (%s)", reason)
		_ = pc.Close()
	}
}
