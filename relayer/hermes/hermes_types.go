package hermes

// ClientCreationResponse contains the minimum required values to extract the client id from the hermes response.
type ClientCreationResponse struct {
	Result ClientResult `json:"result"`
}

type CreateClient struct {
	ClientID string `json:"client_id"`
}

type ClientResult struct {
	CreateClient CreateClient `json:"CreateClient"`
}

// ConnectionResponse contains the minimum required values to extract the connection id from both sides.
type ConnectionResponse struct {
	Result ConnectionResult `json:"result"`
}

type ConnectionResult struct {
	ASide ConnectionSide `json:"a_side"`
	BSide ConnectionSide `json:"b_side"`
}

type ConnectionSide struct {
	ConnectionID string `json:"connection_id"`
}

// ChannelOutputResult contains the minimum required channel values.
type ChannelOutputResult struct {
	Result []ChannelResult `json:"result"`
}

type ChannelResult struct {
	ChannelEnd             ChannelEnd `json:"channel_end"`
	CounterPartyChannelEnd ChannelEnd `json:"counterparty_channel_end"`
}

type ChannelEnd struct {
	ConnectionHops []string         `json:"connection_hops"`
	Ordering       string           `json:"ordering"`
	State          string           `json:"state"`
	Version        string           `json:"version"`
	Remote         ChannelAndPortId `json:"remote"`
}

type ChannelAndPortId struct {
	ChannelID string `json:"channel_id"`
	PortID    string `json:"port_id"`
}
