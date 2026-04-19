package ctlcmd

import (
	"fmt"
	"image"
	"io"
	"os"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/elpdev/pando/internal/identity"
	"github.com/elpdev/pando/internal/invite"
	"github.com/makiuchi-d/gozxing"
	gozxingqr "github.com/makiuchi-d/gozxing/qrcode"
	_ "image/jpeg"
	_ "image/png"
)

type inviteInputOptions struct {
	InvitePath    string
	InviteCode    string
	ReadStdin     bool
	ReadPaste     bool
	ReadClipboard bool
	QRImagePath   string
}

func validateInviteInputFlags(invitePath, inviteCode string, readStdin, readPaste, fromClipboard bool, qrImagePath string) error {
	inputs := 0
	if strings.TrimSpace(invitePath) != "" {
		inputs++
	}
	if strings.TrimSpace(inviteCode) != "" {
		inputs++
	}
	if readStdin {
		inputs++
	}
	if readPaste {
		inputs++
	}
	if fromClipboard {
		inputs++
	}
	if strings.TrimSpace(qrImagePath) != "" {
		inputs++
	}
	if inputs == 0 {
		return fmt.Errorf("provide one of -invite, -code, -stdin, -paste, -from-clipboard, or -qr-image")
	}
	if inputs > 1 {
		return fmt.Errorf("use only one of -invite, -code, -stdin, -paste, -from-clipboard, or -qr-image")
	}
	return nil
}

func readInviteText(input inviteInputOptions) (string, error) {
	switch {
	case strings.TrimSpace(input.InviteCode) != "":
		return input.InviteCode, nil
	case input.ReadClipboard:
		text, err := clipboard.ReadAll()
		if err != nil {
			return "", fmt.Errorf("read invite from clipboard: %w", err)
		}
		return text, nil
	case strings.TrimSpace(input.QRImagePath) != "":
		return readInviteTextFromQRImage(input.QRImagePath)
	case input.ReadStdin || input.ReadPaste || input.InvitePath == "-":
		if input.ReadPaste {
			fmt.Fprintln(os.Stderr, "paste the invite, then press Ctrl-D when finished:")
		}
		bytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read invite from stdin: %w", err)
		}
		return string(bytes), nil
	case strings.TrimSpace(input.InvitePath) != "":
		bytes, err := os.ReadFile(input.InvitePath)
		if err != nil {
			return "", err
		}
		return string(bytes), nil
	default:
		return "", fmt.Errorf("provide one of -invite, -code, -stdin, -paste, -from-clipboard, or -qr-image")
	}
}

func readInviteBundle(input inviteInputOptions) (*identity.InviteBundle, error) {
	text, err := readInviteText(input)
	if err != nil {
		return nil, err
	}
	return invite.DecodeText(text)
}

func readInviteTextFromQRImage(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open QR image: %w", err)
	}
	defer file.Close()
	img, _, err := image.Decode(file)
	if err != nil {
		return "", fmt.Errorf("decode QR image: %w", err)
	}
	bitmap, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return "", fmt.Errorf("read QR image: %w", err)
	}
	result, err := gozxingqr.NewQRCodeReader().Decode(bitmap, nil)
	if err != nil {
		return "", fmt.Errorf("read QR image: %w", err)
	}
	return result.GetText(), nil
}
