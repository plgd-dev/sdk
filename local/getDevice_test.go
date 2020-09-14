package local_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/plgd-dev/sdk/local"

	"github.com/plgd-dev/sdk/schema"
	"github.com/plgd-dev/sdk/test"
	"github.com/stretchr/testify/require"
)

type sortResourcesByHref []schema.ResourceLink

func (a sortResourcesByHref) Len() int      { return len(a) }
func (a sortResourcesByHref) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a sortResourcesByHref) Less(i, j int) bool {
	return a[i].Href < a[j].Href
}

func sortResources(s []schema.ResourceLink) []schema.ResourceLink {
	v := sortResourcesByHref(s)
	sort.Sort(v)
	return v
}

func NewTestDeviceSimulator(deviceID, deviceName string) local.DeviceDetails {
	return local.DeviceDetails{
		ID: deviceID,
		Device: schema.Device{
			ID:            deviceID,
			Name:          deviceName,
			ResourceTypes: []string{"oic.d.cloudDevice", "oic.wk.d"},
			Interfaces:    []string{"oic.if.r", "oic.if.baseline"},
		},
		Resources: sortResources(append(test.TestDevsimResources, test.TestDevsimPrivateResources...)),
	}
}

func NewTestSecureDeviceSimulator(deviceID, deviceName string) local.DeviceDetails {
	return local.DeviceDetails{
		ID: deviceID,
		Device: schema.Device{
			ID:            deviceID,
			Name:          deviceName,
			ResourceTypes: []string{"oic.d.cloudDevice", "oic.wk.d"},
			Interfaces:    []string{"oic.if.r", "oic.if.baseline"},
		},
		IsSecured: true,
		Ownership: &schema.Doxm{
			ResourceOwner:                 "00000000-0000-0000-0000-000000000000",
			SupportedOwnerTransferMethods: []schema.OwnerTransferMethod{schema.JustWorks, schema.ManufacturerCertificate},
			OwnerID:                       "00000000-0000-0000-0000-000000000000",
			DeviceID:                      deviceID,
			SupportedCredentialTypes:      schema.CredentialType(schema.CredentialType_SYMMETRIC_PAIR_WISE | schema.CredentialType_ASYMMETRIC_SIGNING_WITH_CERTIFICATE),
			SelectedOwnerTransferMethod:   schema.JustWorks,
			Interfaces:                    []string{"oic.if.rw", "oic.if.baseline"},
			ResourceTypes:                 []string{"oic.r.doxm"},
		},
		Resources: sortResources(append(append(test.TestDevsimResources, test.TestDevsimPrivateResources...), test.TestDevsimSecResources...)),
	}
}

func cleanUpResources(s []schema.ResourceLink) []schema.ResourceLink {
	a := make([]schema.ResourceLink, 0, len(s))
	for _, l := range s {
		l.Endpoints = nil
		l.Policy = nil
		l.Anchor = ""
		a = append(a, l)
	}
	return a
}

func TestClient_GetDevice(t *testing.T) {
	deviceID := test.MustFindDeviceByName(test.TestDeviceName)
	secureDeviceID := test.MustFindDeviceByName(test.TestSecureDeviceName)
	type args struct {
		deviceID string
	}
	tests := []struct {
		name    string
		args    args
		want    local.DeviceDetails
		wantErr bool
	}{
		{
			name: "valid",
			args: args{
				deviceID: deviceID,
			},
			want: NewTestDeviceSimulator(deviceID, test.TestDeviceName),
		},
		{
			name: "valid - secure",
			args: args{
				deviceID: secureDeviceID,
			},
			want: NewTestSecureDeviceSimulator(secureDeviceID, test.TestSecureDeviceName),
		},
		{
			name: "not-found",
			args: args{
				deviceID: "not-found",
			},
			wantErr: true,
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), TestTimeout)
	defer cancel()

	c := NewTestClient()
	defer c.Close(context.Background())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(ctx, time.Second)
			defer cancel()
			got, err := c.GetDevice(ctx, tt.args.deviceID)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			got.Resources = cleanUpResources(sortResources(got.Resources))
			got.Endpoints = nil
			require.Equal(t, tt.want, got)
		})
	}
}
