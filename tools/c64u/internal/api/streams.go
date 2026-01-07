package api

import (
	"fmt"
)

// Data Streams API (U64 Only)

// StreamsStart starts a video, audio, or debug stream to IP:port
// stream: video, audio, or debug
// ip: destination IP address
// Default ports: video=11000, audio=11001, debug=11002
func (c *Client) StreamsStart(stream, ip string) (*Response, error) {
	params := map[string]string{
		"ip": ip,
	}

	endpoint := fmt.Sprintf("/v1/streams/%s:start", stream)
	return c.Put(endpoint, params)
}

// StreamsStop stops specified stream
// stream: video, audio, or debug
func (c *Client) StreamsStop(stream string) (*Response, error) {
	endpoint := fmt.Sprintf("/v1/streams/%s:stop", stream)
	return c.Put(endpoint, nil)
}
