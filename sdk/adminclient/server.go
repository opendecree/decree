package adminclient

import "context"

// GetServerInfo returns the server's version, commit, and enabled features.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	if c.server == nil {
		return nil, ErrServiceNotConfigured
	}
	return retry(ctx, c, func(ctx context.Context) (*ServerInfo, error) {
		return c.server.GetServerInfo(ctx)
	})
}
