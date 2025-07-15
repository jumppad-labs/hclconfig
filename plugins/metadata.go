package plugins

// Metadata represents plugin metadata information
type Metadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Description  string   `json:"description"`
	Author       string   `json:"author"`
	Homepage     string   `json:"homepage"`
	License      string   `json:"license,omitempty"`
	Capabilities []string `json:"capabilities"`
	API          string   `json:"api"`
	OS           []string `json:"os,omitempty"`
	Arch         []string `json:"arch,omitempty"`
}
