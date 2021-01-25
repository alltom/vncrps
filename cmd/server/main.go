package main

import (
	"bufio"
	"encoding/binary"
	"flag"
	"fmt"
	"github.com/alltom/vncrps/rfb"
	"image"
	"io"
	"log"
	"net"
	"time"
)

const maxFPS = 20

func main() {
	flag.Parse()

	gameServer := NewGameServer(time.Now)

	ln, err := net.Listen("tcp", "127.0.0.1:5900")
	if err != nil {
		log.Fatalf("couldn't listen: %v", err)
	}
	log.Print("listening…")
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("couldn't accept connection: %v", err)
		}
		log.Print("accepted connection")
		go func(conn net.Conn) {
			if err := rfbServe(conn, gameServer); err != nil {
				log.Printf("serve failed: %v", err)
			}
			if err := conn.Close(); err != nil {
				log.Printf("couldn't close connection: %v", err)
			}
		}(conn)
	}
}

func rfbServe(conn io.ReadWriter, gameServer *GameServer) error {
	var bo = binary.BigEndian
	var pixelFormat = rfb.PixelFormat{
		BitsPerPixel: 32,
		BitDepth:     24,
		BigEndian:    true,
		TrueColor:    true,

		RedMax:     255,
		GreenMax:   255,
		BlueMax:    255,
		RedShift:   24,
		GreenShift: 16,
		BlueShift:  8,
	}
	protocolVersion := rfb.ProtocolVersionMessage{3, 3}
	authScheme := rfb.AuthenticationSchemeMessageRFB33{rfb.AuthenticationSchemeVNC}
	var authChallenge rfb.VNCAuthenticationChallengeMessage
	var authResponse rfb.VNCAuthenticationResponseMessage
	authResult := rfb.VNCAuthenticationResultMessage{rfb.VNCAuthenticationResultOK}
	var clientInit rfb.ClientInitialisationMessage
	var serverInit rfb.ServerInitialisationMessage
	var keyEvent rfb.KeyEventMessage
	var pointerEvent rfb.PointerEventMessage

	if err := protocolVersion.Write(conn); err != nil {
		return fmt.Errorf("write ProtocolVersion: %v", err)
	}
	if err := protocolVersion.Read(conn); err != nil {
		return fmt.Errorf("read ProtocolVersion: %v", err)
	}
	if protocolVersion.Major != 3 || protocolVersion.Minor != 3 {
		return fmt.Errorf("only version 3.3 is supported, but client requested %d.%d", protocolVersion.Major, protocolVersion.Minor)
	}

	// Using VNC authentication because the built-in macOS client won't connect otherwise. Accepts any password.
	if err := authScheme.Write(conn, bo); err != nil {
		return fmt.Errorf("write VNC auth scheme: %v", err)
	}
	// Send empty challenge
	if err := authChallenge.Write(conn); err != nil {
		return fmt.Errorf("write VNC auth challenge: %v", err)
	}
	if err := authResponse.Read(conn); err != nil {
		return fmt.Errorf("read VNC auth response: %v", err)
	}
	// Always OK
	if err := authResult.Write(conn, bo); err != nil {
		return fmt.Errorf("write VNC auth result: %v", err)
	}

	if err := clientInit.Read(conn); err != nil {
		return fmt.Errorf("read ClientInitialisation: %v", err)
	}

	serverInit = rfb.ServerInitialisationMessage{
		FramebufferWidth:  uint16(UIWidth),
		FramebufferHeight: uint16(UIHeight),
		PixelFormat:       pixelFormat,
		Name:              "RPS",
	}
	if err := serverInit.Write(conn, bo); err != nil {
		return fmt.Errorf("write ServerInitialisation: %v", err)
	}

	ui := NewUI(gameServer)
	defer ui.Close()

	var nextFrameTime time.Time

	r := bufio.NewReader(conn)
	w := bufio.NewWriter(conn)

	for {
		messageType, err := r.Peek(1)
		if err != nil {
			return fmt.Errorf("read message type: %v", err)
		}
		switch messageType[0] {
		case 0: // SetPixelFormat
			var m rfb.SetPixelFormatMessage
			if err := m.Read(r, bo); err != nil {
				return fmt.Errorf("read SetPixelFormat: %v", err)
			}
			pixelFormat = m.PixelFormat

		case 2: // SetEncodings
			var m rfb.SetEncodingsMessage
			if err := m.Read(r, bo); err != nil {
				return fmt.Errorf("read SetEncodings: %v", err)
			}
			// Nothing to do.

		case 3: // FramebufferUpdateRequest
			var m rfb.FramebufferUpdateRequestMessage
			if err := m.Read(r, bo); err != nil {
				return fmt.Errorf("read FramebufferUpdateRequest: %v", err)
			}

			var update rfb.FramebufferUpdateMessage
			img := rfb.NewPixelFormatImage(pixelFormat, image.Rect(int(m.X), int(m.Y), int(m.X)+int(m.Width), int(m.Y)+int(m.Height)))
			ui.Update(img, &keyEvent, &pointerEvent)
			update.Rectangles = []*rfb.FramebufferUpdateRect{
				&rfb.FramebufferUpdateRect{
					X: m.X, Y: m.Y, Width: m.Width, Height: m.Height,
					EncodingType: 0, PixelData: img.Pix,
				},
			}

			<-time.After(nextFrameTime.Sub(time.Now()))
			if err := update.Write(w, bo); err != nil {
				return fmt.Errorf("write FramebufferUpdate: %v", err)
			}
			if err := w.Flush(); err != nil {
				return fmt.Errorf("flush FramebufferUpdate: %v", err)
			}
			nextFrameTime = time.Now().Add(time.Second / maxFPS)

		case 4: // KeyEvent
			if err := keyEvent.Read(r, bo); err != nil {
				return fmt.Errorf("read KeyEvent: %v", err)
			}
			ui.Update(image.NewNRGBA(image.ZR), &keyEvent, &pointerEvent)

		case 5: // PointerEvent
			if err := pointerEvent.Read(r, bo); err != nil {
				return fmt.Errorf("read PointerEvent: %v", err)
			}
			ui.Update(image.NewNRGBA(image.ZR), &keyEvent, &pointerEvent)

		case 6: // ClientCutText
			var m rfb.ClientCutTextMessage
			if err := m.Read(r, bo); err != nil {
				return fmt.Errorf("read ClientCutText: %v", err)
			}
			// Ignore.

		default:
			return fmt.Errorf("received unrecognized message type %d", messageType[0])
		}
	}

	return nil
}
