package schema

type Attribute struct {
	Name       string       `json:"name,omitempty"`
	Type       string       `json:"type,omitempty"`
	Tags       string       `json:"tags,omitempty"`
	Properties []*Attribute `json:"properties,omitempty"`
}
