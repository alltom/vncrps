package main

import (
	"fmt"
	"github.com/alltom/vncrps/rfb"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"image"
	"image/color"
	"image/draw"
)

const (
	UIWidth        = 320
	UIHeight       = 320
	RankingsSplitX = 240
)

var (
	primaryColor      = color.NRGBA{0x60, 0x02, 0xee, 0xff}
	primaryLightColor = color.NRGBA{0x99, 0x46, 0xff, 0xff}
)

type UI struct {
	server   *GameServer
	playerId PlayerId

	rockButton, paperButton, scissorsButton ButtonState
	move                                    *Move
}

func NewUI(gameServer *GameServer) *UI {
	playerId := gameServer.AddPlayer()
	return &UI{server: gameServer, playerId: playerId}
}

func (ui *UI) Update(img draw.Image, keyEvent *rfb.KeyEventMessage, pointerEvent *rfb.PointerEventMessage) image.Rectangle {
	state, err := ui.server.GetState(ui.playerId)
	if err != nil {
		return image.Rect(0, 0, UIWidth, UIHeight)
	}

	draw.Draw(img, img.Bounds(), image.NewUniform(color.White), image.ZP, draw.Src)

	go func() {
		y := 8
		splitX := (UIHeight + RankingsSplitX) / 2
		for _, player := range state.Rankings {
			name := player.Name
			if player.PlayerId == ui.playerId {
				name += "*"
			}
			label(name, image.Rect(RankingsSplitX+8, y, splitX-8, y+8), img)
			label(fmt.Sprintf("%d", player.Rank), image.Rect(splitX, y, UIWidth-8, y+8), img)
			y += 16
		}
	}()

	switch state.Phase {
	case PhaseWaiting:
		label("Waiting for other players...", image.Rect(8, 8, UIWidth-8, 24), img)
	case PhasePicking:
		draw.Draw(img, image.Rect(0, 0, RankingsSplitX, UIHeight), image.NewUniform(color.RGBA{0xff, 0xff, 0, 0xff}), image.ZP, draw.Src)

		if state.Opponent == nil {
			label("YOU MUST SIT OUT THIS ROUND", image.Rect(8, 8, UIWidth-8, 24), img)
			label("(must be an odd number of players)", image.Rect(8, 32, UIWidth-8, 40), img)
		} else {
			label("CHOOSE YOUR WEAPON", image.Rect(8, 8, UIWidth-8, 24), img)
			rockLabel := "rock"
			paperLabel := "paper"
			scissorsLabel := "scissors"
			if button(&ui.rockButton, rockLabel, image.Rect(8, 32, 77, 64), img, pointerEvent) {
				ui.server.Pick(ui.playerId, MoveRock)
			}
			if button(&ui.paperButton, paperLabel, image.Rect(85, 32, 154, 64), img, pointerEvent) {
				ui.server.Pick(ui.playerId, MovePaper)
			}
			if button(&ui.scissorsButton, scissorsLabel, image.Rect(162, 32, 231, 64), img, pointerEvent) {
				ui.server.Pick(ui.playerId, MoveScissors)
			}

			label(fmt.Sprintf("WHAT WILL %s CHOOSE?", state.Opponent.Name), image.Rect(8, 200, UIWidth-8, 216), img)
		}

		label(fmt.Sprintf("%v left...", state.TimeLeftInPhase), image.Rect(8, 72, UIWidth-8, 88), img)

	case PhaseReview:
		if state.Opponent == nil {
			label("Wait for it...", image.Rect(8, 8, RankingsSplitX-8, 24), img)
		} else {
			mine := "YOUR MOVE: none"
			if state.PlayerMove != nil {
				mine = fmt.Sprintf("YOUR MOVE: %v", state.PlayerMove)
			}
			label(mine, image.Rect(8, 8, RankingsSplitX-8, 24), img)

			theirs := fmt.Sprintf("%s's MOVE: none", state.Opponent.Name)
			if state.OpponentMove != nil {
				theirs = fmt.Sprintf("%s's MOVE: %v", state.Opponent.Name, state.OpponentMove)
			}
			label(theirs, image.Rect(8, 32, RankingsSplitX-8, 48), img)

			winner := "-- there was no winner --"
			if state.Winner != nil {
				if *state.Winner == ui.playerId {
					winner = "YOU WIN!!"
				} else if *state.Winner == state.Opponent.PlayerId {
					winner = "THEY WON!!"
				}
			}
			label(winner, image.Rect(8, 56, RankingsSplitX-8, 72), img)
		}
	}

	return image.Rect(0, 0, UIWidth, UIHeight)
}

func (ui *UI) Close() {
	ui.server.RemovePlayer(ui.playerId)
}

func label(text string, rect image.Rectangle, img draw.Image) {
	fd := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.Black),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{fixed.I(rect.Min.X), fixed.I(rect.Max.Y)},
	}
	fd.DrawString(text)
}

type ButtonState struct {
	clicking bool
}

func button(state *ButtonState, text string, rect image.Rectangle, img draw.Image, pointerEvent *rfb.PointerEventMessage) bool {
	hovering := image.Pt(int(pointerEvent.X), int(pointerEvent.Y)).In(rect)
	buttonDown := pointerEvent.ButtonMask&1 != 0

	// TODO: Require that the click started on the button.
	var clicked bool
	if state.clicking {
		if !buttonDown {
			clicked = hovering
			state.clicking = false
		}
	} else {
		if hovering && buttonDown {
			state.clicking = true
		}
	}

	c := image.Uniform{primaryColor}
	if hovering {
		if buttonDown {
			c.C = color.Black
		} else {
			c.C = primaryLightColor
		}
	}
	draw.Draw(img, rect, &c, image.ZP, draw.Src)

	fd := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.White),
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{fixed.I(rect.Min.X + 8), fixed.I(rect.Max.Y - 8)},
	}
	fd.DrawString(text)

	return clicked
}
