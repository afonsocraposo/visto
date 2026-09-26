package webpush

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	push "github.com/SherClockHolmes/webpush-go"
)

type Client struct {
	PublicKey, PrivateKey, Subject string
	HTTP                           *http.Client
}

func (c *Client) SendPush(ctx context.Context, subscription string, payload []byte) (bool, error) {
	var sub push.Subscription
	if err := json.Unmarshal([]byte(subscription), &sub); err != nil {
		return false, err
	}
	client := c.HTTP
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }, Transport: &http.Transport{DialContext: publicDial}}
	}
	response, err := push.SendNotificationWithContext(ctx, payload, &sub, &push.Options{HTTPClient: client, Subscriber: c.Subject, VAPIDPublicKey: c.PublicKey, VAPIDPrivateKey: c.PrivateKey, TTL: 86400})
	if err != nil {
		return false, err
	}
	defer response.Body.Close()
	io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
	if response.StatusCode == http.StatusGone || response.StatusCode == http.StatusNotFound {
		return true, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return false, fmt.Errorf("web push provider returned HTTP %d", response.StatusCode)
	}
	return false, nil
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	dialer := net.Dialer{Timeout: 10 * time.Second}
	for _, address := range addresses {
		ip := address.IP
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
			continue
		}
		return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
	}
	return nil, errors.New("push endpoint has no public address")
}
