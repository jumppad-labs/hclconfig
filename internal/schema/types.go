package schema

type Attribute struct {
	Name       string       `json:"name,omitempty"`
	Type       string       `json:"type,omitempty"`
	Tags       string       `json:"tags,omitempty"`
	Anonymous  bool         `json:"anonymous,omitempty"`
	Properties []*Attribute `json:"properties,omitempty"`
}
