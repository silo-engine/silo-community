package catalogue

// Manifest represents the structure of a Silo image catalogue, manifest file.
type Manifest struct {
	DocVersion          string    `json:"docVersion"`
	DocName             string    `json:"docName"`
	Distro              string    `json:"distro"`
	Version             string    `json:"version"`
	Name                string    `json:"name"`
	ReleaseVersion      string    `json:"releaseVersion"`
	ImageSize           string    `json:"imageSize"`
	OriginCatalogueName string    `json:"originCatalogueName"`
	Arch                string    `json:"arch"`
	Packages            []Package `json:"packages"`
}

// Package represents a package within a Silo image catalogue, manifest file.
type Package struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}
