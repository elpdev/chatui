package chat

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elpdev/pando/internal/messaging"
	"github.com/elpdev/pando/internal/rendezvous"
)

var generateInviteCode = rendezvous.GenerateCode

func importPasteCmd(service *messaging.Service, text string) tea.Cmd {
	return func() tea.Msg {
		contact, err := service.ImportContactInviteText(text, true)
		if err != nil {
			return addContactImportResultMsg{err: err}
		}
		return addContactImportResultMsg{contact: contact}
	}
}

func lookupContactCmd(service *messaging.Service, ensureRelayClient func() (RelayClient, error), mailbox string) tea.Cmd {
	client, err := ensureRelayClient()
	if err != nil {
		return func() tea.Msg { return addContactLookupResultMsg{err: err} }
	}
	return func() tea.Msg {
		contact, err := service.ImportDirectoryContact(client, mailbox)
		return addContactLookupResultMsg{contact: contact, err: err}
	}
}

func runInviteExchangeCmd(ctx context.Context, service *messaging.Service, ensureRelayClient func() (RelayClient, error), code string) tea.Cmd {
	client, err := ensureRelayClient()
	if err != nil {
		return func() tea.Msg { return addContactInviteExchangeResultMsg{err: err} }
	}
	id := service.Identity()
	return func() tea.Msg {
		bundle, err := rendezvous.Exchange(ctx, rendezvous.PollConfig{
			Client:        client,
			Code:          code,
			Self:          id.InviteBundle(),
			SelfAccountID: id.AccountID,
		})
		if err != nil {
			return addContactInviteExchangeResultMsg{err: err, cancelled: ctx.Err() == context.Canceled}
		}
		contact, err := service.ImportInviteCodeContact(*bundle)
		return addContactInviteExchangeResultMsg{contact: contact, err: err}
	}
}
