//
// Copyright (c) 2018- yutopp (yutopp@gmail.com)
//
// Distributed under the Boost Software License, Version 1.0. (See accompanying
// file LICENSE_1_0.txt or copy at  https://www.boost.org/LICENSE_1_0.txt)
//

package rtmp

import (
	"github.com/sirupsen/logrus"

	"github.com/yutopp/go-rtmp/message"
)

var _ streamHandler = (*controlStreamHandler)(nil)

type controlStreamState uint8

const (
	controlStreamStateNotConnected controlStreamState = iota
	controlStreamStateConnected
)

// controlStreamHandler Handle messages which are categorised as control messages.
//   transitions:
//     = controlStreamStateNotConnected
//       | "connect" -> controlStreamStateConnected
//       | _         -> self
//
//     = controlStreamStateConnected
//       | _ -> self
//
type controlStreamHandler struct {
	conn           *Conn
	state          controlStreamState
	defaultHandler streamHandler

	logger *logrus.Logger
}

func (h *controlStreamHandler) Handle(chunkStreamID int, timestamp uint32, msg message.Message, stream *Stream) error {
	switch h.state {
	case controlStreamStateNotConnected:
		return h.handleConnect(chunkStreamID, timestamp, msg, stream)
	case controlStreamStateConnected:
		return h.handleCreateStream(chunkStreamID, timestamp, msg, stream)
	default:
		panic("Unreachable!")
	}
}

func (h *controlStreamHandler) handleConnect(chunkStreamID int, timestamp uint32, msg message.Message, stream *Stream) error {
	l := h.logger.WithFields(logrus.Fields{
		"stream_id": stream.streamID,
		"state":     h.state,
		"handler":   "control",
	})

	var cmdMsgWrapper amfWrapperFunc
	var cmdMsg *message.CommandMessage
	switch msg := msg.(type) {
	case *message.CommandMessageAMF0:
		cmdMsgWrapper = amf0Wrapper
		cmdMsg = &msg.CommandMessage
		goto handleCommand

	case *message.CommandMessageAMF3:
		cmdMsgWrapper = amf0Wrapper
		cmdMsg = &msg.CommandMessage
		goto handleCommand

	default:
		l.Info("Message unhandled: Msg = %+v", msg)
		return h.defaultHandler.Handle(chunkStreamID, timestamp, msg, stream)
	}

handleCommand:
	switch cmd := cmdMsg.Command.(type) {
	case *message.NetConnectionConnect:
		l.Info("Connect")

		if err := h.conn.handler.OnConnect(timestamp, cmd); err != nil {
			return err
		}

		// TODO: fix
		if err := stream.Write(chunkStreamID, timestamp, &message.WinAckSize{
			Size: h.conn.streamer.selfState.windowSize,
		}); err != nil {
			return err
		}

		// TODO: fix
		if err := stream.Write(chunkStreamID, timestamp, &message.SetPeerBandwidth{
			Size:  1 * 1024 * 1024,
			Limit: 1,
		}); err != nil {
			return err
		}

		// TODO: fix
		m := cmdMsgWrapper(func(cmsg *message.CommandMessage) {
			*cmsg = message.CommandMessage{
				CommandName:   "_result",
				TransactionID: 1, // 7.2.1.2, flow.6
				Command: &message.NetConnectionConnectResult{
					Properties: message.NetConnectionConnectResultProperties{
						FMSVer:       "rtmp/testing",
						Capabilities: 250,
						Mode:         1,
					},
					Information: message.NetConnectionConnectResultInformation{
						Level: "status",
						Code:  "NetConnection.Connect.Success",
						Data: map[string]interface{}{
							"version": "testing",
						},
						Application: nil,
					},
				},
			}
		})
		l.Infof("Conn: %+v", m.(*message.CommandMessageAMF0).Command)

		if err := stream.Write(chunkStreamID, timestamp, m); err != nil {
			return err
		}
		l.Info("Connected")

		h.state = controlStreamStateConnected

		return nil

	default:
		l.Infof("Unexpected command: Command = %+v", cmdMsg)
		return nil
	}

}

func (h *controlStreamHandler) handleCreateStream(chunkStreamID int, timestamp uint32, msg message.Message, stream *Stream) error {
	l := h.logger.WithFields(logrus.Fields{
		"stream_id": stream.streamID,
		"state":     h.state,
		"handler":   "control",
	})

	var cmdMsgWrapper amfWrapperFunc
	var cmdMsg *message.CommandMessage
	switch msg := msg.(type) {
	case *message.CommandMessageAMF0:
		cmdMsgWrapper = amf0Wrapper
		cmdMsg = &msg.CommandMessage
		goto handleCommand

	case *message.CommandMessageAMF3:
		cmdMsgWrapper = amf0Wrapper
		cmdMsg = &msg.CommandMessage
		goto handleCommand

	default:
		l.Infof("Message unhandled: Msg = %+v", msg)
		return h.defaultHandler.Handle(chunkStreamID, timestamp, msg, stream)
	}

handleCommand:
	switch cmd := cmdMsg.Command.(type) {
	case *message.NetConnectionCreateStream:
		l.Infof("Stream creating...: %+v", cmd)

		// Create a stream which handles messages for data(play, publish, video, audio, etc...)
		streamID, err := h.conn.createStreamIfAvailable(&dataStreamHandler{
			conn:           h.conn,
			defaultHandler: h.defaultHandler,
			logger:         h.logger,
		})
		if err != nil {
			// TODO: send failed _result
			l.Errorf("Stream creating...: Err = %+v", err)

			return nil
		}

		// TODO: fix
		m := cmdMsgWrapper(func(cmsg *message.CommandMessage) {
			*cmsg = message.CommandMessage{
				CommandName:   "_result",
				TransactionID: cmdMsg.TransactionID,
				Command: &message.NetConnectionCreateStreamResult{
					StreamID: streamID,
				},
			}
		})
		if err := stream.Write(chunkStreamID, timestamp, m); err != nil {
			_ = h.conn.deleteStream(streamID) // TODO: error handling
			return err
		}

		l.Infof("Stream created...: NewStreamID = %d", streamID)

		return nil

	case *message.NetStreamDeleteStream:
		l.Infof("Stream deleting...: TargetStreamID = %d", cmd.StreamID)

		if err := h.conn.deleteStream(cmd.StreamID); err != nil {
			return err
		}

		// server does not send any response(7.2.2.3)

		l.Infof("Stream deleted: TargetStreamID = %d", cmd.StreamID)

		return nil

	default:
		l.Infof("Unexpected command: Command = %+v", cmdMsg)
		return nil
	}
}
