package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/elpdev/pando/internal/messaging"
	"github.com/elpdev/pando/internal/ui/style"
)

type filePickerEntry struct {
	Name     string
	Path     string
	IsDir    bool
	IsParent bool
	Size     int64
}

type filePickerVisibleEntry struct {
	index int
	entry filePickerEntry
}

type filePickerState struct {
	open     bool
	dir      string
	entries  []filePickerEntry
	selected int
}

func defaultFilePickerDir() string {
	if dir, err := os.Getwd(); err == nil && dir != "" {
		return dir
	}
	if dir, err := os.UserHomeDir(); err == nil && dir != "" {
		return dir
	}
	return string(filepath.Separator)
}

func (m *Model) sendAttachment(path, attachmentType string) tea.Cmd {
	var (
		batch       *messaging.OutgoingBatch
		displayBody string
		err         error
	)
	switch attachmentType {
	case messaging.AttachmentTypePhoto:
		batch, displayBody, err = m.messaging.PreparePhotoOutgoing(m.recipientMailbox, path)
	case messaging.AttachmentTypeVoice:
		batch, displayBody, err = m.messaging.PrepareVoiceOutgoing(m.recipientMailbox, path)
	case messaging.AttachmentTypeFile:
		batch, displayBody, err = m.messaging.PrepareFileOutgoing(m.recipientMailbox, path)
	default:
		m.pushToast(fmt.Sprintf("unsupported attachment type %q", attachmentType), ToastBad)
		return nil
	}
	if err != nil {
		m.pushToast(err.Error(), ToastBad)
		return nil
	}
	m.appendMessageItem(messageItem{
		direction:    "outbound",
		sender:       m.mailbox,
		body:         displayBody,
		timestamp:    time.Now().UTC(),
		messageID:    batchMessageID(batch),
		status:       statusPending,
		isAttachment: true,
	})
	m.input.SetValue("")
	m.resetLocalTypingState()
	m.syncViewportToBottom()
	return m.sendCmd(m.recipientMailbox, displayBody, batch)
}

func (m *Model) updateFilePicker(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		m.closeFilePicker()
		return nil
	case tea.KeyBackspace:
		if err := m.goToParentDirectory(); err != nil {
			m.pushToast(fmt.Sprintf("file picker failed: %v", err), ToastBad)
		}
		return nil
	case tea.KeyUp:
		m.moveFilePickerSelection(-1)
		return nil
	case tea.KeyDown:
		m.moveFilePickerSelection(1)
		return nil
	case tea.KeyEnter:
		entry := m.selectedFilePickerEntry()
		if entry == nil {
			return nil
		}
		if entry.IsDir {
			if err := m.openFilePickerAt(entry.Path); err != nil {
				m.pushToast(fmt.Sprintf("open directory failed: %v", err), ToastBad)
			}
			return nil
		}
		m.closeFilePicker()
		return m.sendAttachment(entry.Path, messaging.AttachmentTypeFile)
	}
	return nil
}

func (m *Model) openFilePicker() error {
	return m.openFilePickerAt(m.filePicker.dir)
}

func (m *Model) openFilePickerAt(dir string) error {
	entries, cleanedDir, err := readFilePickerEntries(dir)
	if err != nil {
		return err
	}
	m.filePicker.open = true
	m.filePicker.dir = cleanedDir
	m.filePicker.entries = entries
	m.filePicker.selected = 0
	m.input.Blur()
	return nil
}

func (m *Model) closeFilePicker() {
	m.filePicker.open = false
	m.filePicker.entries = nil
	m.filePicker.selected = 0
	m.input.Focus()
}

func (m *Model) goToParentDirectory() error {
	parent := filepath.Dir(m.filePicker.dir)
	if parent == m.filePicker.dir {
		return nil
	}
	return m.openFilePickerAt(parent)
}

func (m *Model) moveFilePickerSelection(delta int) {
	if len(m.filePicker.entries) == 0 {
		return
	}
	m.filePicker.selected += delta
	if m.filePicker.selected < 0 {
		m.filePicker.selected = 0
	}
	if m.filePicker.selected >= len(m.filePicker.entries) {
		m.filePicker.selected = len(m.filePicker.entries) - 1
	}
}

func (m *Model) selectedFilePickerEntry() *filePickerEntry {
	if m.filePicker.selected < 0 || m.filePicker.selected >= len(m.filePicker.entries) {
		return nil
	}
	return &m.filePicker.entries[m.filePicker.selected]
}

func readFilePickerEntries(dir string) ([]filePickerEntry, string, error) {
	cleanedDir := filepath.Clean(dir)
	entries, err := os.ReadDir(cleanedDir)
	if err != nil {
		return nil, "", err
	}
	items := make([]filePickerEntry, 0, len(entries)+1)
	for _, entry := range entries {
		name := entry.Name()
		var size int64
		if !entry.IsDir() {
			if info, err := entry.Info(); err == nil {
				size = info.Size()
			}
		}
		items = append(items, filePickerEntry{
			Name:  name,
			Path:  filepath.Join(cleanedDir, name),
			IsDir: entry.IsDir(),
			Size:  size,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].IsDir != items[j].IsDir {
			return items[i].IsDir
		}
		return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
	})
	if parent := filepath.Dir(cleanedDir); parent != cleanedDir {
		items = append([]filePickerEntry{{
			Name:     "..",
			Path:     parent,
			IsDir:    true,
			IsParent: true,
		}}, items...)
	}
	return items, cleanedDir, nil
}

func formatFileSize(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)
	switch {
	case bytes < KB:
		return fmt.Sprintf("%d B", bytes)
	case bytes < MB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/MB)
	default:
		return fmt.Sprintf("%.1f GB", float64(bytes)/GB)
	}
}

func (m *Model) renderFilePicker(width int) string {
	title := style.Bold.Render("Attach File")
	dirLine := style.Muted.Render(m.filePicker.dir)
	hint := style.Muted.Render("enter open/select  |  backspace up  |  esc cancel")
	lines := []string{title, dirLine, hint, ""}
	modalWidth := min(max(48, width-6), width)
	rowWidth := max(1, modalWidth-6)
	visibleEntries, hiddenAbove, hiddenBelow := m.filePickerVisibleEntries(max(1, m.height-12))
	if len(m.filePicker.entries) == 0 {
		lines = append(lines, style.Muted.Render("(empty) — backspace to go up"))
	} else {
		if hiddenAbove {
			lines = append(lines, style.Muted.Render("..."))
		}
		for _, visible := range visibleEntries {
			lines = append(lines, m.renderFilePickerRow(visible.entry, visible.index == m.filePicker.selected, rowWidth))
		}
		if hiddenBelow {
			lines = append(lines, style.Muted.Render("..."))
		}
	}
	modalHeight := max(8, m.height-4)
	modal := style.ModalBorder.Padding(1).Width(max(1, modalWidth-4)).Height(max(1, modalHeight-4)).Render(strings.Join(lines, "\n"))
	return lipgloss.Place(width, max(1, m.height), lipgloss.Center, lipgloss.Center, modal)
}

func (m *Model) renderFilePickerRow(entry filePickerEntry, selected bool, width int) string {
	label := entry.Name
	if entry.IsDir {
		label += string(filepath.Separator)
	}
	sizeStr := ""
	if !entry.IsDir {
		sizeStr = formatFileSize(entry.Size)
	}
	pad := width - lipgloss.Width(label) - lipgloss.Width(sizeStr)
	if pad < 1 {
		pad = 1
	}
	line := label + strings.Repeat(" ", pad) + style.Muted.Render(sizeStr)

	rowStyle := lipgloss.NewStyle().Width(width)
	if entry.IsDir {
		rowStyle = rowStyle.Inherit(style.StatusOk)
	}
	if selected {
		rowStyle = rowStyle.Inherit(style.Selected).Bold(true)
	}
	return rowStyle.Render(line)
}

func (m *Model) filePickerVisibleEntries(maxEntries int) ([]filePickerVisibleEntry, bool, bool) {
	if len(m.filePicker.entries) == 0 {
		return nil, false, false
	}
	if maxEntries <= 0 || len(m.filePicker.entries) <= maxEntries {
		visible := make([]filePickerVisibleEntry, 0, len(m.filePicker.entries))
		for idx, entry := range m.filePicker.entries {
			visible = append(visible, filePickerVisibleEntry{index: idx, entry: entry})
		}
		return visible, false, false
	}
	start := m.filePicker.selected - (maxEntries / 2)
	if start < 0 {
		start = 0
	}
	end := start + maxEntries
	if end > len(m.filePicker.entries) {
		end = len(m.filePicker.entries)
		start = end - maxEntries
	}
	visible := make([]filePickerVisibleEntry, 0, end-start)
	for idx := start; idx < end; idx++ {
		visible = append(visible, filePickerVisibleEntry{index: idx, entry: m.filePicker.entries[idx]})
	}
	return visible, start > 0, end < len(m.filePicker.entries)
}
