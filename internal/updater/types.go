package updater

type Status struct {
	Available      bool   `json:"available"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	ReleaseURL     string `json:"release_url,omitempty"`
	ReleaseNotes   string `json:"release_notes,omitempty"`
	CanApply       bool   `json:"can_apply"`
	Message        string `json:"message,omitempty"`
}

type ApplyResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}
