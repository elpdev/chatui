package chat

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elpdev/pando/internal/messaging"
)

type voiceNoteOption struct {
	id        string
	filename  string
	mimeType  string
	path      string
	timestamp time.Time
	direction string
	size      int64
}

func (m *Model) recentVoiceNotes() []voiceNoteOption {
	if m.peer.mailbox == "" || m.peer.isRoom {
		return nil
	}
	options := make([]voiceNoteOption, 0)
	for i := len(m.msgs.items) - 1; i >= 0; i-- {
		item := m.msgs.items[i]
		if item.kind != transcriptMessage || item.attachment == nil {
			continue
		}
		if item.attachment.Type != messaging.AttachmentTypeVoice || strings.TrimSpace(item.attachment.LocalPath) == "" {
			continue
		}
		filename := item.attachment.Filename
		if filename == "" {
			filename = filepath.Base(item.attachment.LocalPath)
		}
		id := fmt.Sprintf("%d:%s:%s", item.timestamp.UnixNano(), item.attachment.LocalPath, filename)
		options = append(options, voiceNoteOption{
			id:        id,
			filename:  filename,
			mimeType:  item.attachment.MIMEType,
			path:      item.attachment.LocalPath,
			timestamp: item.timestamp,
			direction: item.direction,
			size:      item.attachment.Size,
		})
	}
	return options
}

func (m *Model) findVoiceNote(id string) *voiceNoteOption {
	for _, note := range m.recentVoiceNotes() {
		if note.id == id {
			return &note
		}
	}
	return nil
}

func (m *Model) playVoiceNoteCmd(id string) tea.Cmd {
	note := m.findVoiceNote(id)
	if note == nil {
		m.pushToast("voice note no longer available", ToastWarn)
		return nil
	}
	return func() tea.Msg {
		data, err := m.messaging.AttachmentBytes(note.path)
		if err != nil {
			return voicePlaybackResultMsg{filename: note.filename, err: fmt.Errorf("read voice note: %w", err)}
		}
		if err := m.voicePlayer.Play(note.filename, note.mimeType, data); err != nil {
			return voicePlaybackResultMsg{filename: note.filename, err: err}
		}
		return voicePlaybackResultMsg{filename: note.filename}
	}
}

func (m *Model) stopVoiceNotePlayback() tea.Cmd {
	if m.voicePlayer == nil || !m.voicePlayer.IsPlaying() {
		m.pushToast("no voice note is playing", ToastInfo)
		return nil
	}
	if err := m.voicePlayer.Stop(); err != nil {
		m.pushToast(fmt.Sprintf("stop playback failed: %v", err), ToastBad)
		return nil
	}
	m.pushToast("voice note playback stopped", ToastInfo)
	return nil
}

func formatVoiceNoteDetail(note voiceNoteOption) string {
	direction := "received"
	if note.direction == "outbound" {
		direction = "sent"
	}
	detail := fmt.Sprintf("%s %s", direction, note.timestamp.Local().Format("Mon 3:04 PM"))
	if note.size > 0 {
		detail += "  " + formatFileSize(note.size)
	}
	return detail
}
