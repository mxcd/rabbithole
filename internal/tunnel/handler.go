package tunnel

import (
	"encoding/json"

	"github.com/mxcd/rabbithole/pkg/protocol"
	"github.com/rs/zerolog/log"
)

func (t *Tunnel) ReadLoop() {
	defer func() {
		log.Info().Str("subdomain", t.Subdomain).Msg("tunnel read loop ended")
	}()

	for {
		_, raw, err := t.Connection.ReadMessage()
		if err != nil {
			log.Debug().Err(err).Str("subdomain", t.Subdomain).Msg("tunnel read error")
			return
		}

		var msg protocol.Message
		if err := json.Unmarshal(raw, &msg); err != nil {
			log.Warn().Err(err).Str("subdomain", t.Subdomain).Msg("failed to unmarshal message")
			continue
		}

		switch msg.Type {
		case protocol.TypeHTTPResponse:
			var resp protocol.HTTPResponse
			if err := json.Unmarshal(raw, &resp); err != nil {
				log.Warn().Err(err).Msg("failed to unmarshal HTTP response")
				continue
			}
			t.ResolvePendingRequest(resp.ID, &resp)

		case protocol.TypeWSOpened:
			var opened protocol.WSOpened
			if err := json.Unmarshal(raw, &opened); err != nil {
				log.Warn().Err(err).Msg("failed to unmarshal WSOpened")
				continue
			}
			t.resolveWSOpened(opened.StreamID)

		case protocol.TypeWSFrame:
			var frame protocol.WSFrame
			if err := json.Unmarshal(raw, &frame); err != nil {
				log.Warn().Err(err).Msg("failed to unmarshal WSFrame")
				continue
			}
			t.deliverWSFrame(&frame)

		case protocol.TypeWSClose:
			var close protocol.WSClose
			if err := json.Unmarshal(raw, &close); err != nil {
				log.Warn().Err(err).Msg("failed to unmarshal WSClose")
				continue
			}
			t.closeWSStream(close.StreamID)

		case protocol.TypePong:
			// heartbeat response, nothing to do

		default:
			log.Warn().Str("type", string(msg.Type)).Msg("unknown message type from tunnel client")
		}
	}
}

func (t *Tunnel) resolveWSOpened(streamID string) {
	t.WsMu.Lock()
	stream, ok := t.WsStreams[streamID]
	t.WsMu.Unlock()
	if ok && stream.OpenedChan != nil {
		select {
		case stream.OpenedChan <- struct{}{}:
		default:
		}
	}
}

func (t *Tunnel) deliverWSFrame(frame *protocol.WSFrame) {
	t.WsMu.Lock()
	stream, ok := t.WsStreams[frame.StreamID]
	t.WsMu.Unlock()
	if ok {
		select {
		case stream.FrameChan <- frame:
		case <-stream.Done:
		}
	}
}

func (t *Tunnel) closeWSStream(streamID string) {
	t.WsMu.Lock()
	stream, ok := t.WsStreams[streamID]
	if ok {
		delete(t.WsStreams, streamID)
	}
	t.WsMu.Unlock()
	if ok {
		close(stream.Done)
	}
}

func (t *Tunnel) RegisterWSStream(streamID string, stream *WSStream) {
	t.WsMu.Lock()
	defer t.WsMu.Unlock()
	t.WsStreams[streamID] = stream
}

func (t *Tunnel) DeregisterWSStream(streamID string) {
	t.WsMu.Lock()
	stream, ok := t.WsStreams[streamID]
	if ok {
		delete(t.WsStreams, streamID)
	}
	t.WsMu.Unlock()
	if ok {
		select {
		case <-stream.Done:
		default:
			close(stream.Done)
		}
	}
}
