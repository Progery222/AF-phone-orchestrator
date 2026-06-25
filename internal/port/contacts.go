package port

import "context"

type ContactGroup struct {
	Name       string
	ContactIDs []string
}

type PhoneContactsState struct {
	Serial         string
	ContactsCount  int32
	Groups         []string
	Status         string
	Source         string
	Message        string
}

type Contact struct {
	ID          string
	Serial      string
	DisplayName string
	Phones      []string
	Emails      []string
	Groups      []string
}

type ContactsClient interface {
	Upload(ctx context.Context, serial, source string, groupFilter []string, vcardKey string) (PhoneContactsState, error)
	Sync(ctx context.Context, serial string) (PhoneContactsState, error)
	Merge(ctx context.Context, serial string) (PhoneContactsState, error)
	ApplyGroups(ctx context.Context, serial string, groups []ContactGroup) (PhoneContactsState, error)
	Export(ctx context.Context, serial, format string) ([]byte, string, error)
	ListContacts(ctx context.Context, serial string) ([]Contact, PhoneContactsState, error)
	DeleteContact(ctx context.Context, contactID string) error
	Ping(ctx context.Context) error
}
