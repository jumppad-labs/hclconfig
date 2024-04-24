package registry

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type Registry interface {
	GetModuleVersions(organization string, module string) (*Versions, error)
	GetModule(organization string, name string, version string) (*Module, error)
}

type TransportWithCredentials struct {
	token string
	T     http.RoundTripper
}

func (t *TransportWithCredentials) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	return t.T.RoundTrip(req)
}

type RegistryImpl struct {
	client  http.Client
	Host    string
	Modules string
}

type Config struct {
	Capabilities map[string]string `json:"capabilities"`
}

type Credential struct {
	Token string `hcl:"token,optional" json:"token,omitempty"`
}

type Module struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Organization string `json:"organization"`
	Namespace    string `json:"namespace"` // implement later? public/private modules
	Version      string `json:"version"`
	SourceURL    string `json:"source_url"`
	DownloadURL  string `json:"download_url"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type Versions struct {
	Latest   string    `json:"latest"`
	Versions []Version `json:"versions"`
}

type Version struct {
	Version   string `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func New(host string, token string) (Registry, error) {
	client := http.Client{
		Timeout: 5 * time.Second,
		Transport: &TransportWithCredentials{
			token: token,
			T:     http.DefaultTransport,
		},
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/.well-known/registry.json", host), nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf(`"%s" is not a valid registry`, host)
	}

	var config Config
	err = json.NewDecoder(resp.Body).Decode(&config)
	if err != nil {
		return nil, err
	}

	if config.Capabilities["modules.v1"] == "" {
		return nil, fmt.Errorf(`registry "%s" does not support modules`, host)
	}

	parsedURL, err := url.Parse(config.Capabilities["modules.v1"])
	if err != nil {
		return nil, err
	}

	// if the modules url also contains a host, use that instead
	if parsedURL.Host != "" {
		host = parsedURL.Host
	}

	return &RegistryImpl{
		client:  client,
		Host:    host,
		Modules: host + parsedURL.Path,
	}, nil
}

func (r *RegistryImpl) GetModuleVersions(organization string, module string) (*Versions, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/%s/%s/versions", r.Modules, organization, module), nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	var versions Versions
	err = json.NewDecoder(resp.Body).Decode(&versions)
	if err != nil {
		return nil, err
	}

	return &versions, nil
}

func (r *RegistryImpl) GetModule(organization string, name string, version string) (*Module, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("http://%s/%s/%s/%s", r.Modules, organization, name, version), nil)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.Do(req)
	if err != nil {
		return nil, err
	}

	var module Module
	err = json.NewDecoder(resp.Body).Decode(&module)
	if err != nil {
		return nil, err
	}

	return &module, nil
}
