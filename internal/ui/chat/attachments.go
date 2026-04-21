package chat

import (
	"os"
	"strings"

	"github.com/elpdev/pando/internal/messaging"
	"github.com/elpdev/pando/internal/store"
)

func legacyAttachmentRecord(body string) *store.AttachmentRecord {
	for _, attachmentType := range []string{messaging.AttachmentTypePhoto, messaging.AttachmentTypeVoice, messaging.AttachmentTypeFile} {
		label := messaging.AttachmentLabel(attachmentType)
		sentPrefix := label + " sent: "
		if strings.HasPrefix(body, sentPrefix) {
			return messaging.NewAttachmentRecord(attachmentType, strings.TrimSpace(strings.TrimPrefix(body, sentPrefix)), "", "", 0)
		}
		receivedPrefix := label + " received: "
		if !strings.HasPrefix(body, receivedPrefix) {
			continue
		}
		rest := strings.TrimSpace(strings.TrimPrefix(body, receivedPrefix))
		filename, path, ok := strings.Cut(rest, " saved to ")
		if !ok {
			return messaging.NewAttachmentRecord(attachmentType, strings.TrimSpace(rest), "", "", 0)
		}
		path = strings.TrimSpace(path)
		if _, err := os.Stat(path); err != nil {
			path = ""
		}
		return messaging.NewAttachmentRecord(attachmentType, strings.TrimSpace(filename), "", path, 0)
	}
	return nil
}

func isPhotoAttachment(attachment *store.AttachmentRecord) bool {
	return attachment != nil && attachment.Type == messaging.AttachmentTypePhoto && attachment.LocalPath != ""
}
