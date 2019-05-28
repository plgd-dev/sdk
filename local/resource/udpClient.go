package resource

import (
	"context"

	gocoap "github.com/go-ocf/go-coap"
	"github.com/go-ocf/sdk/kit/coap"
	"github.com/go-ocf/sdk/kit/net"
	"github.com/go-ocf/sdk/local/resource/link"
	"github.com/go-ocf/sdk/schema"
)

// UDPClientFactory maintains the shared link cache and connection pool.
type UDPClientFactory struct {
	linkCache *link.Cache
	pool      *coap.Pool
}

// NewUDPClientFactory creates the client factory.
func NewUDPClientFactory(linkCache *link.Cache) *UDPClientFactory {
	udpPool := coap.NewPool(createUDPConnection)
	return &UDPClientFactory{linkCache: linkCache, pool: udpPool}
}

// NewClient populates the link cache and the connection pool,
// then creates the client that uses the shared link cache and connection pool.
func (f *UDPClientFactory) NewClient(
	c *gocoap.ClientConn,
	links schema.DeviceLinks,
	codec Codec,
) (*Client, error) {
	f.linkCache.Put(links.ID, links.Links...)
	addr, err := net.Parse(c.RemoteAddr())
	if err != nil {
		return nil, err
	}
	f.pool.Put(addr, c)
	return f.NewClientFromCache(codec)
}

// NewClientFromCache creates the client
// that uses the shared link cache and connection pool.
func (f *UDPClientFactory) NewClientFromCache(codec Codec) (*Client, error) {
	c := Client{
		linkCache: f.linkCache,
		pool:      f.pool,
		codec:     codec,
		getAddr:   getUDPAddr,
	}
	return &c, nil
}

func getUDPAddr(r *schema.ResourceLink) (net.Addr, error) {
	return r.GetUDPAddr()
}

func createUDPConnection(ctx context.Context, p *coap.Pool, a net.Addr) error {
	closeSession := func(error) { p.Delete(a) }
	client := gocoap.Client{Net: "udp", NotifySessionEndFunc: closeSession}
	c, err := client.DialWithContext(ctx, a.String())
	if err != nil {
		return err
	}
	p.Put(a, c)
	return nil
}