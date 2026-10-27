package session

// Metadata holds information about the Silo session, including details about the base image used to create the session.
type Metadata struct {
	BaseImage *BaseImageMetadata
}

// BaseImageMetadata holds information about the base image used to create the Silo session.
type BaseImageMetadata struct {
	Distro          string `json:"distro"`
	Version         string `json:"version"`
	ReleaseVersion  string `json:"release_version"`
	ReleaseChecksum string `json:"release_checksum"`
	Description     string `json:"description"`
	ImageURL        string `json:"image_url"`
	ArchString      string `json:"arch_string"` 
}

// NewMetadata creates a new Metadata instance with the provided base image metadata.
func NewMetadata(baseImage *BaseImageMetadata) *Metadata {
	return &Metadata{
		BaseImage: baseImage,
	}
}

// NewBaseImageMetadata creates a new BaseImageMetadata instance with the provided details about the base image.
func NewBaseImageMetadata(
	distro,
	version,
	releaseVersion,
	releaseChecksum,
	description,
	imageURL,
	archString string) *BaseImageMetadata {

	return &BaseImageMetadata{
		Distro:          distro,
		Version:         version,
		ReleaseVersion:  releaseVersion,
		ReleaseChecksum: releaseChecksum,
		Description:     description,
		ImageURL:        imageURL,
		ArchString:      archString,
	}
}
