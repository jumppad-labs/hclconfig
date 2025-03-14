package schema

type Entity struct {
	Name       string    `json:"name,omitempty"`
	Type       string    `json:"type,omitempty"`
	Tags       string    `json:"tags,omitempty"`
	Properties []*Entity `json:"properties,omitempty"`
}
