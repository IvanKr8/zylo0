package volume

type Volume struct {
	Name    string   `json:"name"`
	Path    string   `json:"path"`
	Created string   `json:"created"`
	UsedBy  []string `json:"used_by"`
}

type Registry struct {
	Volumes []Volume `json:"volumes"`
}
