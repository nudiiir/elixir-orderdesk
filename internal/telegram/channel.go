package telegram

import (
	"context"
	"errors"
)

// Messenger is everything the desk needs from a chat channel. Telegram is one; WhatsApp is another
// (package whatsapp), which speaks the same messages and buttons.
type Messenger interface {
	Send(ctx context.Context, chatID int64, text string, rows [][]Button) (int, error)
	Edit(ctx context.Context, chatID int64, messageID int, text string, rows [][]Button) error
	Answer(ctx context.Context, callbackID, text string)
	Typing(ctx context.Context, chatID int64)
	Download(ctx context.Context, fileID string) ([]byte, error)
}

// WAOffset puts WhatsApp chats in their own range of chat ids: a WhatsApp chat is the number's
// digits plus WAOffset (E.164 numbers have at most 15 digits; Telegram ids are far below).
const WAOffset int64 = 2_000_000_000_000_000

func IsWhatsApp(chatID int64) bool { return chatID >= WAOffset }

// Mux sends each chat's messages through its own channel.
type Mux struct {
	TG *Telegram
	WA Messenger
}

var errNoChannel = errors.New("no channel for this chat")

func (m *Mux) pick(chatID int64) Messenger {
	if IsWhatsApp(chatID) {
		if m.WA != nil {
			return m.WA
		}
		return nil
	}
	if m.TG != nil {
		return m.TG
	}
	return nil
}

func (m *Mux) Send(ctx context.Context, chatID int64, text string, rows [][]Button) (int, error) {
	if c := m.pick(chatID); c != nil {
		return c.Send(ctx, chatID, text, rows)
	}
	return 0, errNoChannel
}

func (m *Mux) Edit(ctx context.Context, chatID int64, messageID int, text string, rows [][]Button) error {
	if c := m.pick(chatID); c != nil {
		return c.Edit(ctx, chatID, messageID, text, rows)
	}
	return errNoChannel
}

// Answer acknowledges a button press: only Telegram has the notion; a WhatsApp reply needs none.
func (m *Mux) Answer(ctx context.Context, callbackID, text string) {
	if m.TG != nil && callbackID != "" && !isWamid(callbackID) {
		m.TG.Answer(ctx, callbackID, text)
	}
}

func (m *Mux) Typing(ctx context.Context, chatID int64) {
	if c := m.pick(chatID); c != nil {
		c.Typing(ctx, chatID)
	}
}

// Download fetches a file by the id its channel gave it. WhatsApp media ids are prefixed "wa:" by
// the whatsapp package so that they reach the right channel.
func (m *Mux) Download(ctx context.Context, fileID string) ([]byte, error) {
	if len(fileID) > 3 && fileID[:3] == "wa:" {
		if m.WA == nil {
			return nil, errNoChannel
		}
		return m.WA.Download(ctx, fileID)
	}
	if m.TG == nil {
		return nil, errNoChannel
	}
	return m.TG.Download(ctx, fileID)
}

func isWamid(id string) bool { return len(id) > 5 && id[:5] == "wamid" }
