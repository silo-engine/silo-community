package catalogue

// SiloImageCatalogue represents a catalogue of Silo images.
type SiloImageCatalogue struct {
	Version           string     `json:"version"`
	Name              string     `json:"name"`
	Description       string     `json:"description"`
	DefaultArm64Image *SiloImage `json:"defaultArm64Image,omitempty"` //only applies to the official base image catalogue, which is expected to be named "silo-base-image-catalogue"
	DefaultAmd64Image *SiloImage `json:"defaultAmd64Image,omitempty"` //only applies to the official base image catalogue, which is expected to be named "silo-base-image-catalogue"
	SiloImages        SiloImages `json:"silo-images"`
}

func (c *SiloImageCatalogue) GetDefaultImage(archType ArchType) *SiloImage {
	switch archType {
	case Arm64:
		return c.DefaultArm64Image
	case Amd64:
		return c.DefaultAmd64Image
	default:
		return nil
	}
}

func (c *SiloImageCatalogue) ListImages(archType ArchType) []SiloImage {
	switch archType {
	case Arm64:
		return c.SiloImages.Arm64
	case Amd64:
		return c.SiloImages.Amd64
	default:
		return nil
	}
}

func (c *SiloImageCatalogue) GetImageByID(archType ArchType, id string) *SiloImage {
	images := c.ListImages(archType)
	for _, image := range images {
		if image.ReleaseVersion == id {
			return &image
		}
	}
	return nil
}

func (c *SiloImageCatalogue) GetImageByDistroAndVersion(archType ArchType, distro string, version string) *SiloImage {
	images := c.ListImages(archType)
	for _, image := range images {
		if image.Distro == distro && image.Version == version {
			return &image
		}
	}
	return nil
}

// SiloImage represents a single Silo image in the catalogue.
type SiloImage struct {
	Distro          string `json:"distro"`
	Version         string `json:"version"`
	ReleaseVersion  string `json:"release_version"`
	ReleaseChecksum string `json:"release_checksum"`
	Description     string `json:"description"`
	ImageURL        string `json:"image_url"`
	installed       bool
	archString      string
}

func (i *SiloImage) IsInstalled() bool {
	return i.installed
}

func (i *SiloImage) SetInstalled(installed bool) {
	i.installed = installed
}

func (i *SiloImage) GetArchString() string {
	return i.archString
}

func (i *SiloImage) SetArchString(archString string) {
	i.archString = archString
}

// SiloImages represents a collection of Silo images for different architectures.
type SiloImages struct {
	Arm64 []SiloImage `json:"arm64"`
	Amd64 []SiloImage `json:"amd64"`
}
