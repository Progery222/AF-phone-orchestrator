package driver

import (
	"context"

	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
)

type StubContacts struct{}

func NewStubContacts() *StubContacts { return &StubContacts{} }

func (s *StubContacts) Upload(ctx context.Context, serial, source string, groupFilter []string, vcardKey string) (port.PhoneContactsState, error) {
	return port.PhoneContactsState{Serial: serial, ContactsCount: 1, Status: "PHONE_CONTACTS_STATUS_SYNCED", Message: "stub upload"}, nil
}

func (s *StubContacts) Sync(ctx context.Context, serial string) (port.PhoneContactsState, error) {
	return port.PhoneContactsState{Serial: serial, Status: "PHONE_CONTACTS_STATUS_SYNCED", Source: "CONTACT_SOURCE_GOOGLE"}, nil
}

func (s *StubContacts) Merge(ctx context.Context, serial string) (port.PhoneContactsState, error) {
	return port.PhoneContactsState{Serial: serial, Status: "PHONE_CONTACTS_STATUS_SYNCED"}, nil
}

func (s *StubContacts) ApplyGroups(ctx context.Context, serial string, groups []port.ContactGroup) (port.PhoneContactsState, error) {
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Name
	}
	return port.PhoneContactsState{Serial: serial, Groups: names, Status: "PHONE_CONTACTS_STATUS_SYNCED"}, nil
}

func (s *StubContacts) Export(ctx context.Context, serial, format string) ([]byte, string, error) {
	return []byte("BEGIN:VCARD\nEND:VCARD\n"), "vcard", nil
}

func (s *StubContacts) ListContacts(ctx context.Context, serial string) ([]port.Contact, port.PhoneContactsState, error) {
	return nil, port.PhoneContactsState{Serial: serial, Status: "PHONE_CONTACTS_STATUS_PENDING"}, nil
}

func (s *StubContacts) DeleteContact(ctx context.Context, contactID string) error { return nil }

func (s *StubContacts) Ping(context.Context) error { return nil }

var _ port.ContactsClient = (*StubContacts)(nil)
