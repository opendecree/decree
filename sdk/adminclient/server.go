package adminclient

import "context"

// GetServerInfo returns the server's version, commit, and enabled features.
func (c *Client) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	if c.server == nil {
		return nil, ErrServiceNotConfigured
	}
	return c.server.GetServerInfo(ctx)
}
