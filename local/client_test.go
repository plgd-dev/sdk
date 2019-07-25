package local_test

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	ocfSigner "github.com/go-ocf/kit/security/signer"
	ocf "github.com/go-ocf/sdk/local"
	"github.com/go-ocf/sdk/schema"
)

type Client struct {
	*ocf.Client
	otm *ocf.ManufacturerOTMClient

	DeviceID string
	*ocf.Device
}

func NewTestSecureClient() (*Client, error) {
	identityCert, err := tls.X509KeyPair(IdentityCert, IdentityKey)
	if err != nil {
		return nil, err
	}
	return NewTestSecureClientWithCert(identityCert)
}

func NewTestSecureClientWithCert(cert tls.Certificate) (*Client, error) {
	mfgCert, err := tls.X509KeyPair(MfgCert, MfgKey)
	if err != nil {
		return nil, err
	}
	mfgTrustedCABlock, _ := pem.Decode(MfgTrustedCA)
	if mfgTrustedCABlock == nil {
		return nil, fmt.Errorf("mfgTrustedCABlock is empty")
	}
	mfgCa, err := x509.ParseCertificate(mfgTrustedCABlock.Bytes)
	if err != nil {
		return nil, err
	}

	identityIntermediateCABlock, _ := pem.Decode(IdentityIntermediateCA)
	if identityIntermediateCABlock == nil {
		return nil, fmt.Errorf("identityIntermediateCABlock is empty")
	}
	identityIntermediateCA, err := x509.ParseCertificates(identityIntermediateCABlock.Bytes)
	if err != nil {
		return nil, err
	}
	identityIntermediateCAKeyBlock, _ := pem.Decode(IdentityIntermediateCAKey)
	if identityIntermediateCAKeyBlock == nil {
		return nil, fmt.Errorf("identityIntermediateCAKeyBlock is empty")
	}
	identityIntermediateCAKey, err := x509.ParseECPrivateKey(identityIntermediateCAKeyBlock.Bytes)
	if err != nil {
		return nil, err
	}

	identityTrustedCABlock, _ := pem.Decode(IdentityTrustedCA)
	if identityTrustedCABlock == nil {
		return nil, fmt.Errorf("identityTrustedCABlock is empty")
	}
	identityTrustedCA, err := x509.ParseCertificates(identityTrustedCABlock.Bytes)
	if err != nil {
		return nil, err
	}

	signer := ocfSigner.NewIdentityCertificateSigner(identityIntermediateCA, identityIntermediateCAKey, time.Hour*86400)

	otm := ocf.NewManufacturerOTMClient(mfgCert, mfgCa, signer, identityTrustedCA)
	if err != nil {
		return nil, err
	}

	c := ocf.NewClient(ocf.WithTLS(&ocf.TLSConfig{
		GetCertificate: func() (tls.Certificate, error) {
			return cert, nil
		},
		GetCertificateAuthorities: func() ([]*x509.Certificate, error) {
			cas := identityTrustedCA
			cas = append(cas, mfgCa)
			return cas, nil
		},
	}))

	return &Client{Client: c, otm: otm}, nil
}

func (c *Client) SetUpTestDevice(t *testing.T) {
	id := testGetDeviceID(t, c.Client, true)

	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	device, _, err := c.GetDevice(timeout, id)
	require.NoError(t, err)
	err = device.Own(timeout, c.otm)
	require.NoError(t, err)
	c.Device = device
	c.DeviceID = id
}

func (c *Client) Close() {
	if c.DeviceID == "" {
		return
	}
	timeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := c.Disown(timeout)
	if err != nil {
		panic(err)
	}
	c.Device.Close(timeout)
}

type testFindDeviceHandler struct {
	secured bool
	t       *testing.T

	lock      sync.Mutex
	deviceIds map[string]bool
}

func (h *testFindDeviceHandler) Handle(ctx context.Context, d *ocf.Device, links schema.ResourceLinks) {
	secured, err := d.IsSecured(ctx)
	require.NoError(h.t, err)
	defer d.Close(ctx)
	if secured != h.secured {
		return
	}
	h.lock.Lock()
	defer h.lock.Unlock()
	if h.deviceIds == nil {
		h.deviceIds = make(map[string]bool)
	}
	h.deviceIds[d.DeviceID()] = true
}

func (h *testFindDeviceHandler) PopDeviceIds() map[string]bool {
	h.lock.Lock()
	defer h.lock.Unlock()
	tmp := h.deviceIds
	h.deviceIds = nil
	return tmp
}

func (h *testFindDeviceHandler) DeviceIDs() []string {
	h.lock.Lock()
	defer h.lock.Unlock()

	out := make([]string, 0, len(h.deviceIds))
	for id, _ := range h.deviceIds {
		out = append(out, id)
	}
	return out
}

func (h *testFindDeviceHandler) Error(err error) {
}

func testGetDeviceID(t *testing.T, c *ocf.Client, secured bool) string {
	timeout, cancel := context.WithTimeout(context.Background(), time.Second*1)
	defer cancel()
	h := testFindDeviceHandler{secured: secured, t: t}
	err := c.GetDevices(timeout, &h)
	require.NoError(t, err)
	deviceIds := h.PopDeviceIds()
	require.NotEmpty(t, deviceIds)
	require.Len(t, deviceIds, 1)
	for key, _ := range deviceIds {
		return key
	}
	return ""
}

var (
	CertIdentity = "b5a2a42e-b285-42f1-a36b-034c8fc8efd5"

	MfgCert = []byte(`-----BEGIN CERTIFICATE-----
MIIB9zCCAZygAwIBAgIRAOwIWPAt19w7DswoszkVIEIwCgYIKoZIzj0EAwIwEzER
MA8GA1UEChMIVGVzdCBPUkcwHhcNMTkwNTAyMjAwNjQ4WhcNMjkwMzEwMjAwNjQ4
WjBHMREwDwYDVQQKEwhUZXN0IE9SRzEyMDAGA1UEAxMpdXVpZDpiNWEyYTQyZS1i
Mjg1LTQyZjEtYTM2Yi0wMzRjOGZjOGVmZDUwWTATBgcqhkjOPQIBBggqhkjOPQMB
BwNCAAQS4eiM0HNPROaiAknAOW08mpCKDQmpMUkywdcNKoJv1qnEedBhWne7Z0jq
zSYQbyqyIVGujnI3K7C63NRbQOXQo4GcMIGZMA4GA1UdDwEB/wQEAwIDiDAzBgNV
HSUELDAqBggrBgEFBQcDAQYIKwYBBQUHAwIGCCsGAQUFBwMBBgorBgEEAYLefAEG
MAwGA1UdEwEB/wQCMAAwRAYDVR0RBD0wO4IJbG9jYWxob3N0hwQAAAAAhwR/AAAB
hxAAAAAAAAAAAAAAAAAAAAAAhxAAAAAAAAAAAAAAAAAAAAABMAoGCCqGSM49BAMC
A0kAMEYCIQDuhl6zj6gl2YZbBzh7Th0uu5izdISuU/ESG+vHrEp7xwIhANCA7tSt
aBlce+W76mTIhwMFXQfyF3awWIGjOcfTV8pU
-----END CERTIFICATE-----
`)

	MfgKey = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIMPeADszZajrkEy4YvACwcbR0pSdlKG+m8ALJ6lj/ykdoAoGCCqGSM49
AwEHoUQDQgAEEuHojNBzT0TmogJJwDltPJqQig0JqTFJMsHXDSqCb9apxHnQYVp3
u2dI6s0mEG8qsiFRro5yNyuwutzUW0Dl0A==
-----END EC PRIVATE KEY-----
`)

	MfgTrustedCA = []byte(`-----BEGIN CERTIFICATE-----
MIIBaTCCAQ+gAwIBAgIQR33gIB75I7Vi/QnMnmiWvzAKBggqhkjOPQQDAjATMREw
DwYDVQQKEwhUZXN0IE9SRzAeFw0xOTA1MDIyMDA1MTVaFw0yOTAzMTAyMDA1MTVa
MBMxETAPBgNVBAoTCFRlc3QgT1JHMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE
xbwMaS8jcuibSYJkCmuVHfeV3xfYVyUq8Iroz7YlXaTayspW3K4hVdwIsy/5U+3U
vM/vdK5wn2+NrWy45vFAJqNFMEMwDgYDVR0PAQH/BAQDAgEGMBMGA1UdJQQMMAoG
CCsGAQUFBwMBMA8GA1UdEwEB/wQFMAMBAf8wCwYDVR0RBAQwAoIAMAoGCCqGSM49
BAMCA0gAMEUCIBWkxuHKgLSp6OXDJoztPP7/P5VBZiwLbfjTCVRxBvwWAiEAnzNu
6gKPwtKmY0pBxwCo3NNmzNpA6KrEOXE56PkiQYQ=
-----END CERTIFICATE-----
`)
	MfgTrustedCAKey = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEICzfC16AqtSv3wt+qIbrgM8dTqBhHANJhZS5xCpH6P2roAoGCCqGSM49
AwEHoUQDQgAExbwMaS8jcuibSYJkCmuVHfeV3xfYVyUq8Iroz7YlXaTayspW3K4h
VdwIsy/5U+3UvM/vdK5wn2+NrWy45vFAJg==
-----END EC PRIVATE KEY-----
`)

	IdentityTrustedCA = []byte(`-----BEGIN CERTIFICATE-----
MIIBaDCCAQ6gAwIBAgIRANpzWRKheR25RH0CgYYwLzQwCgYIKoZIzj0EAwIwETEP
MA0GA1UEAxMGUm9vdENBMCAXDTE5MDcxOTEzMTA1M1oYDzIxMTkwNjI1MTMxMDUz
WjARMQ8wDQYDVQQDEwZSb290Q0EwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAASQ
TLfEiNgEfqyWmtW1RV9UKgxsMddrNlYFt/+ZpqaJpBQ+hvtGwJenLEv5jzeEcMXr
gOR4EwjjJSzELk6IibC+o0UwQzAOBgNVHQ8BAf8EBAMCAQYwEwYDVR0lBAwwCgYI
KwYBBQUHAwEwDwYDVR0TAQH/BAUwAwEB/zALBgNVHREEBDACggAwCgYIKoZIzj0E
AwIDSAAwRQIhAOUfsOKyjIgYmDd2G46ge+PEPAZ9DS67Q5RjJvLk/lf3AiA6yMxJ
msmj2nz8VeEkxpKq3gYwJUdJ9jMklTzP+Dcenw==
-----END CERTIFICATE-----
`)
	IdentityTrustedCAKey = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIFe+pAuS4dEt1gRZ6Mq1xrgkEHxL191AFzEsNNvTEWOYoAoGCCqGSM49
AwEHoUQDQgAEkEy3xIjYBH6slprVtUVfVCoMbDHXazZWBbf/maamiaQUPob7RsCX
pyxL+Y83hHDF64DkeBMI4yUsxC5OiImwvg==
-----END EC PRIVATE KEY-----
`)
	IdentityIntermediateCA = []byte(`-----BEGIN CERTIFICATE-----
MIIBczCCARmgAwIBAgIRANntjEpzu9krzL0EG6fcqqgwCgYIKoZIzj0EAwIwETEP
MA0GA1UEAxMGUm9vdENBMCAXDTE5MDcxOTIwMzczOVoYDzIxMTkwNjI1MjAzNzM5
WjAZMRcwFQYDVQQDEw5JbnRlcm1lZGlhdGVDQTBZMBMGByqGSM49AgEGCCqGSM49
AwEHA0IABKw1/6WHFcWtw67hH5DzoZvHgA0suC6IYLKms4IP/pds9wU320eDaENo
5860TOyKrGn7vW/cj/OVe2Dzr4KSFVijSDBGMA4GA1UdDwEB/wQEAwIBBjATBgNV
HSUEDDAKBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/AgEAMAsGA1UdEQQEMAKC
ADAKBggqhkjOPQQDAgNIADBFAiEAgPtnYpgwxmPhN0Mo8VX582RORnhcdSHMzFjh
P/li1WwCIFVVWBOrfBnTt7A6UfjP3ljAyHrJERlMauQR+tkD/aqm
-----END CERTIFICATE-----
`)
	IdentityIntermediateCAKey = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIPF4DPvFeiRL1G0ROd6MosoUGnvIG/2YxH0CbHwnLKxqoAoGCCqGSM49
AwEHoUQDQgAErDX/pYcVxa3DruEfkPOhm8eADSy4Lohgsqazgg/+l2z3BTfbR4No
Q2jnzrRM7Iqsafu9b9yP85V7YPOvgpIVWA==
-----END EC PRIVATE KEY-----
`)
	IdentityCert = []byte(`-----BEGIN CERTIFICATE-----
MIIBsTCCAVagAwIBAgIQaxAoemzXSnFWCq/DmVwQIDAKBggqhkjOPQQDAjAZMRcw
FQYDVQQDEw5JbnRlcm1lZGlhdGVDQTAgFw0xOTA3MTkyMDM3NTFaGA8yMTE5MDYy
NTIwMzc1MVowNDEyMDAGA1UEAxMpdXVpZDowMDAwMDAwMC0wMDAwLTAwMDAtMDAw
MC0wMDAwMDAwMDAwMDEwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAS/gWdMe96z
qsKMOfWsGJtH0wQCRYcwbu0dr+IkQv4/tSv+wO0EVhfvaIAr8lM2xZ6X+uGMcg/Y
muqOL/nFhadlo2MwYTAOBgNVHQ8BAf8EBAMCA4gwMwYDVR0lBCwwKgYIKwYBBQUH
AwEGCCsGAQUFBwMCBggrBgEFBQcDAQYKKwYBBAGC3nwBBjAMBgNVHRMBAf8EAjAA
MAwGA1UdEQQFMAOCATowCgYIKoZIzj0EAwIDSQAwRgIhAJwukCJJtkbgrgrS96uR
RILQxW0aF8K6+5j+CBeEkwYNAiEAguOX+W1WEu1gAf6pIxMOIF83/X4adJd4KEYs
7gMgO3Y=
-----END CERTIFICATE-----
-----BEGIN CERTIFICATE-----
MIIBczCCARmgAwIBAgIRANntjEpzu9krzL0EG6fcqqgwCgYIKoZIzj0EAwIwETEP
MA0GA1UEAxMGUm9vdENBMCAXDTE5MDcxOTIwMzczOVoYDzIxMTkwNjI1MjAzNzM5
WjAZMRcwFQYDVQQDEw5JbnRlcm1lZGlhdGVDQTBZMBMGByqGSM49AgEGCCqGSM49
AwEHA0IABKw1/6WHFcWtw67hH5DzoZvHgA0suC6IYLKms4IP/pds9wU320eDaENo
5860TOyKrGn7vW/cj/OVe2Dzr4KSFVijSDBGMA4GA1UdDwEB/wQEAwIBBjATBgNV
HSUEDDAKBggrBgEFBQcDATASBgNVHRMBAf8ECDAGAQH/AgEAMAsGA1UdEQQEMAKC
ADAKBggqhkjOPQQDAgNIADBFAiEAgPtnYpgwxmPhN0Mo8VX582RORnhcdSHMzFjh
P/li1WwCIFVVWBOrfBnTt7A6UfjP3ljAyHrJERlMauQR+tkD/aqm
-----END CERTIFICATE-----
`)
	IdentityKey = []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEICLgYlcG6V0LbI3IqUENYuVLR2s0Tyqkxz0t1+QP2KVLoAoGCCqGSM49
AwEHoUQDQgAEv4FnTHves6rCjDn1rBibR9MEAkWHMG7tHa/iJEL+P7Ur/sDtBFYX
72iAK/JTNsWel/rhjHIP2Jrqji/5xYWnZQ==
-----END EC PRIVATE KEY-----
`)
)
