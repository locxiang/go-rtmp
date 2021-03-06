package main

import (
	"bytes"
	"errors"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
	"log"
	"net"
)

type Handler struct {
}

func (h *Handler) OnConnect(timestamp uint32, cmd *rtmpmsg.NetConnectionConnect) error {
	log.Printf("OnConnect: %+v", cmd)
	return nil
}

func (h *Handler) OnPublish(timestamp uint32, cmd *rtmpmsg.NetStreamPublish) error {
	log.Printf("OnPublish: %+v", cmd)

	return nil
}

func (h *Handler) OnPlay(timestamp uint32, args []interface{}) error {
	return errors.New("Not supported")
}

func (h *Handler) OnAudio(timestamp uint32, payload []byte) error {
	buf := bytes.NewBuffer(payload)
	audio, err := flvtag.DecodeAudioData(buf)
	if err != nil {
		return err
	}

	log.Printf("FLV Audio Data: Timestamp = %d, SoundFormat = %+v, SoundRate = %+v, SoundSize = %+v, SoundType = %+v, AACPacketType = %+v, Data length = %+v",
		timestamp,
		audio.SoundFormat,
		audio.SoundRate,
		audio.SoundSize,
		audio.SoundType,
		audio.AACPacketType,
		len(audio.Data),
	)

	return nil
}

func (h *Handler) OnVideo(timestamp uint32, payload []byte) error {
	buf := bytes.NewBuffer(payload)
	video, err := flvtag.DecodeVideoData(buf)
	if err != nil {
		return err
	}

	log.Printf("FLV Video Data: Timestamp = %d, FrameType = %+v, CodecID = %+v, AVCPacketType = %+v, CT = %+v, Data length = %+v",
		timestamp,
		video.FrameType,
		video.CodecID,
		video.AVCPacketType,
		video.CompositionTime,
		len(video.Data),
	)

	return nil
}

func main() {
	tcpAddr, err := net.ResolveTCPAddr("tcp", ":1935")
	if err != nil {
		log.Panicf("Failed: %+v", err)
	}

	listner, err := net.ListenTCP("tcp", tcpAddr)
	if err != nil {
		log.Panicf("Failed: %+v", err)
	}

	srv := rtmp.NewServer(&rtmp.ServerConfig{
		HandlerFactory: func() rtmp.Handler {
			return &Handler{}
		},
		Conn: nil,
	})
	if err := srv.Serve(listner); err != nil {
		log.Panicf("Failed: %+v", err)
	}
}
