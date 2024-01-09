package pod

type NgxBackendGroupSpec struct {
	BackendGroup []NgxBackendGroup `json:"backend_group,omitempty"`
}

type NgxBackendGroup struct {
	Name     string       `json:"name"`
	Backends []NgxBackend `json:"backends,omitempty"`
}

type NgxBackend struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
	// Weight  int    `json:"weight,omitempty"`
}
