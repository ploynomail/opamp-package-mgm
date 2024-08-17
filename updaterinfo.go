package opamppackagemgm

type UpdatePackageInfo struct {
	Version        string
	DownloadUrl    string `json:"download_url,omitempty"`
	ContentHash    []byte `json:"content_hash,omitempty"`
	Signature      []byte `json:"signature,omitempty"`
	CurrentVersion string `json:"current_version,omitempty"`
	IsPatch        bool   `json:"is_patch,omitempty"`
}
