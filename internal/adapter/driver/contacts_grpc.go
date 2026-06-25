package driver

import (
	"context"
	"fmt"
	"strings"
	"time"

	contactsv1 "github.com/mobilefarm/af/contacts-manager/gen/contacts/v1"
	"github.com/mobilefarm/af/phone-orchestrator/internal/config"
	"github.com/mobilefarm/af/phone-orchestrator/internal/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

type ContactsGRPC struct {
	client contactsv1.ContactsServiceClient
	conn   *grpc.ClientConn
}

func NewContactsGRPC(cfg config.Config) (*ContactsGRPC, func(), error) {
	conn, err := grpc.NewClient(cfg.ContactsGRPCAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, nil, err
	}
	return &ContactsGRPC{
		client: contactsv1.NewContactsServiceClient(conn),
		conn:   conn,
	}, func() { _ = conn.Close() }, nil
}

func (c *ContactsGRPC) Upload(ctx context.Context, serial, source string, groupFilter []string, vcardKey string) (port.PhoneContactsState, error) {
	st, err := c.client.Upload(ctx, &contactsv1.UploadRequest{
		Serial:      serial,
		Source:      parseContactSource(source),
		GroupFilter: groupFilter,
		VcardKey:    vcardKey,
	})
	if err != nil {
		return port.PhoneContactsState{}, err
	}
	return fromProtoState(st), nil
}

func (c *ContactsGRPC) Sync(ctx context.Context, serial string) (port.PhoneContactsState, error) {
	st, err := c.client.Sync(ctx, &contactsv1.SyncRequest{Serial: serial})
	if err != nil {
		return port.PhoneContactsState{}, err
	}
	return fromProtoState(st), nil
}

func (c *ContactsGRPC) Merge(ctx context.Context, serial string) (port.PhoneContactsState, error) {
	st, err := c.client.Merge(ctx, &contactsv1.MergeRequest{Serial: serial})
	if err != nil {
		return port.PhoneContactsState{}, err
	}
	return fromProtoState(st), nil
}

func (c *ContactsGRPC) ApplyGroups(ctx context.Context, serial string, groups []port.ContactGroup) (port.PhoneContactsState, error) {
	pg := make([]*contactsv1.ContactGroup, len(groups))
	for i, g := range groups {
		pg[i] = &contactsv1.ContactGroup{Name: g.Name, ContactIds: g.ContactIDs}
	}
	st, err := c.client.ApplyGroups(ctx, &contactsv1.ApplyGroupsRequest{Serial: serial, Groups: pg})
	if err != nil {
		return port.PhoneContactsState{}, err
	}
	return fromProtoState(st), nil
}

func (c *ContactsGRPC) Export(ctx context.Context, serial, format string) ([]byte, string, error) {
	resp, err := c.client.Export(ctx, &contactsv1.ExportRequest{Serial: serial, Format: format})
	if err != nil {
		return nil, "", err
	}
	return resp.GetData(), resp.GetFormat(), nil
}

func (c *ContactsGRPC) ListContacts(ctx context.Context, serial string) ([]port.Contact, port.PhoneContactsState, error) {
	resp, err := c.client.ListContacts(ctx, &contactsv1.ListContactsRequest{Serial: serial})
	if err != nil {
		return nil, port.PhoneContactsState{}, err
	}
	out := make([]port.Contact, 0, len(resp.GetContacts()))
	for _, ct := range resp.GetContacts() {
		out = append(out, port.Contact{
			ID: ct.GetId(), Serial: ct.GetSerial(), DisplayName: ct.GetDisplayName(),
			Phones: ct.GetPhones(), Emails: ct.GetEmails(), Groups: ct.GetGroups(),
		})
	}
	state := fromProtoState(resp.GetState())
	state.Serial = serial
	return out, state, nil
}

func (c *ContactsGRPC) DeleteContact(ctx context.Context, contactID string) error {
	_, err := c.client.DeleteContact(ctx, &contactsv1.DeleteContactRequest{ContactId: contactID})
	return err
}

func (c *ContactsGRPC) Ping(ctx context.Context) error {
	if c.conn == nil {
		return fmt.Errorf("contacts-manager: нет соединения")
	}
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		switch c.conn.GetState() {
		case connectivity.Ready, connectivity.Idle:
			return nil
		case connectivity.Shutdown:
			return fmt.Errorf("contacts-manager: соединение закрыто")
		default:
			if !c.conn.WaitForStateChange(ctx, c.conn.GetState()) {
				return fmt.Errorf("contacts-manager: недоступен")
			}
		}
	}
}

func fromProtoState(st *contactsv1.PhoneContactsState) port.PhoneContactsState {
	if st == nil {
		return port.PhoneContactsState{}
	}
	return port.PhoneContactsState{
		Serial: st.GetSerial(), ContactsCount: st.GetContactsCount(), Groups: st.GetGroups(),
		Status: st.GetStatus().String(), Source: st.GetSource().String(), Message: st.GetMessage(),
	}
}

var _ port.ContactsClient = (*ContactsGRPC)(nil)

func parseContactSource(source string) contactsv1.ContactSource {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case "vcard":
		return contactsv1.ContactSource_CONTACT_SOURCE_VCARD
	case "google":
		return contactsv1.ContactSource_CONTACT_SOURCE_GOOGLE
	case "sim":
		return contactsv1.ContactSource_CONTACT_SOURCE_SIM
	case "db":
		return contactsv1.ContactSource_CONTACT_SOURCE_DB
	default:
		return contactsv1.ContactSource_CONTACT_SOURCE_UNSPECIFIED
	}
}
