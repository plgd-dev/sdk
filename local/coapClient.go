package local

import (
	"context"
	"fmt"
	"strconv"
	"time"
	"strings"
	"crypto/tls"
	"crypto/x509"
	uuid "github.com/gofrs/uuid"

	gocoap "github.com/go-ocf/go-coap"
	"github.com/go-ocf/kit/codec/coap"
	"github.com/go-ocf/kit/net"
	"github.com/go-ocf/sdk/local/resource"
	"github.com/go-ocf/sdk/schema"
)

type CoapClient struct {
	clientConn *gocoap.ClientConn
}

type optionFunc func() string

func VerifyIndetityCertificate(cert *x509.Certificate) error {
	// verify EKU manually
	ekuHasClient := false
	ekuHasServer := false
	for _, eku := range cert.ExtKeyUsage {
		if eku == x509.ExtKeyUsageClientAuth {
			ekuHasClient = true
		}
		if eku == x509.ExtKeyUsageServerAuth {
			ekuHasServer = true
		}
	}
	if !ekuHasClient {
		return fmt.Errorf("not contains ExtKeyUsageClientAuth")
	}
	if !ekuHasServer {
		return fmt.Errorf("not contains ExtKeyUsageServerAuth")
	}
	ekuHasOcfId := false
	for _, eku := range cert.UnknownExtKeyUsage {
		if eku.Equal(ekuOcfId) {
			ekuHasOcfId = true
			break
		}
	}
	if !ekuHasOcfId {
		return fmt.Errorf("not contains ExtKeyUsage with OCF ID(1.3.6.1.4.1.44924.1.6")
	}
	cn := strings.Split(cert.Subject.CommonName, ":")
	if len(cn) != 2 {
		return fmt.Errorf("invalid subject common name: %v", cert.Subject.CommonName)
	}
	if strings.ToLower(cn[0]) != "uuid" {
		return fmt.Errorf("invalid subject common name %v: 'uuid' - not found", cert.Subject.CommonName)
	}
	_, err := uuid.FromString(cn[1])
	if err != nil {
		return fmt.Errorf("invalid subject common name %v: %v", cert.Subject.CommonName, err)
	}
	return nil
}

func DialTcpTls(ctx context.Context, addr string, cert tls.Certificate, cas []*x509.Certificate, verifyPeerCertificate func(verifyPeerCertificate *x509.Certificate) error) (*CoapClient, error) {
	caPool := x509.NewCertPool()
	for _, ca := range cas {
		caPool.AddCert(ca)
	}

	tlsConfig := tls.Config{
		InsecureSkipVerify: true,
		Certificates:       []tls.Certificate{cert},
		VerifyPeerCertificate: func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
			for _, rawCert := range rawCerts {
				cert, err := x509.ParseCertificate(rawCert)
				if err != nil {
					return err
				}
				_, err = cert.Verify(x509.VerifyOptions{
					Roots:       caPool,
					CurrentTime: time.Now(),
					KeyUsages:   []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
				})
				if err != nil {
					return err
				}
				if verifyPeerCertificate(cert) != nil {
					return err
				}
			}
			return nil
		},
	}
	coapConn, err := gocoap.DialWithTLS("tcp", addr, &tlsConfig)
	if err != nil {
		return nil, err
	}
	return NewCoapClient(coapConn), nil
}


func NewCoapClient(clientConn *gocoap.ClientConn) *CoapClient {
	return &CoapClient{clientConn: clientConn}
}

func WithInterface(in string) optionFunc {
	return func() string {
		return "if=" + in
	}
}

func WithType(in string) optionFunc {
	return func() string {
		return "rt=" + in
	}
}

func WithCredentialId(in int) optionFunc {
	return func() string {
		return "credid=" + strconv.Itoa(in)
	}
}

func WithCredentialSubject(in string) optionFunc {
	return func() string {
		return "subjectuuid=" + in
	}
}

func (c *CoapClient) UpdateResourceCBOR(
	ctx context.Context,
	href string,
	request interface{},
	response interface{},
	options ...optionFunc,
) error {
	return c.UpdateResource(ctx, href, coap.CBORCodec{}, request, response, options...)
}

func (c *CoapClient) UpdateResource(
	ctx context.Context,
	href string,
	codec resource.Codec,
	request interface{},
	response interface{},
	options ...optionFunc,
) error {
	var opts []func(gocoap.Message)
	for _, opt := range options {
		opts = append(opts, func(req gocoap.Message) {
			req.AddOption(gocoap.URIQuery, opt())
		})
	}

	return resource.COAPPost(ctx, c.clientConn, href, codec, request, response, opts...)
}

func (c *CoapClient) GetResourceCBOR(
	ctx context.Context,
	href string,
	response interface{},
	options ...optionFunc,
) error {
	return c.GetResource(ctx, href, coap.CBORCodec{}, response, options...)
}

func (c *CoapClient) GetResource(
	ctx context.Context,
	href string,
	codec resource.Codec,
	response interface{},
	options ...optionFunc,
) error {
	var opts []func(gocoap.Message)
	for _, opt := range options {
		opts = append(opts, func(req gocoap.Message) {
			req.AddOption(gocoap.URIQuery, opt())
		})
	}

	return resource.COAPGet(ctx, c.clientConn, href, codec, response, opts...)
}

func (c *CoapClient) GetDeviceLinks(ctx context.Context, deviceID string) (device schema.DeviceLinks, _ error) {
	var devices []schema.DeviceLinks
	err := c.GetResourceCBOR(ctx, "/oic/res", &devices)
	if err != nil {
		return device, err
	}
	for _, d := range devices {
		if d.ID == deviceID {
			device = d
		}
	}
	if device.ID != deviceID {
		return device, fmt.Errorf("cannot get device links: not found")
	}

	links := make([]schema.ResourceLink, 0, len(device.Links))
	for _, link := range device.Links {
		addr, err := net.Parse(c.clientConn.RemoteAddr())
		if err != nil {
			return device, fmt.Errorf("invalid address of device %s: %v", device.ID, err)
		}
		links = append(links, link.PatchEndpoint(addr))
	}
	device.Links = links

	return device, nil
}

func (c *CoapClient) DeleteResource(
	ctx context.Context,
	href string,
	codec resource.Codec,
	response interface{},
	options ...optionFunc,
) error {
	var opts []func(gocoap.Message)
	for _, opt := range options {
		opts = append(opts, func(req gocoap.Message) {
			req.AddOption(gocoap.URIQuery, opt())
		})
	}

	return resource.COAPDelete(ctx, c.clientConn, href, codec, response, opts...)
}

func (c *CoapClient) DeleteResourceCBOR(
	ctx context.Context,
	href string,
	response interface{},
	options ...optionFunc,
) error {
	return c.DeleteResource(ctx, href, coap.CBORCodec{}, response, options...)
}