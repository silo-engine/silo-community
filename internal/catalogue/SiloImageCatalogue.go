package catalogue

import "github.com/silo-engine/silo-community/internal/common"

// SiloImageCatalogue represents a catalogue of Silo images.
type SiloImageCatalogue struct {
	Version           string      `json:"version"`
	Name              string      `json:"name"`
	Description       string      `json:"description"`
	DefaultArm64Image *SiloImage  `json:"defaultArm64Image,omitempty"` //only applies to the official base image catalogue, which is expected to be named "silo-base-image-catalogue"
	DefaultAmd64Image *SiloImage  `json:"defaultAmd64Image,omitempty"` //only applies to the official base image catalogue, which is expected to be named "silo-base-image-catalogue"
	SiloImages        *SiloImages `json:"silo-images"`
}

func (c *SiloImageCatalogue) GetDefaultImage(archType common.ArchType) *SiloImage {
	switch archType {
	case common.Arm64:
		return c.DefaultArm64Image
	case common.Amd64:
		return c.DefaultAmd64Image
	default:
		return nil
	}
}

func (c *SiloImageCatalogue) ListImages(archType common.ArchType, includeEOL bool) []*SiloImage {
	switch archType {
	case common.Arm64:
		if !includeEOL {
			var nonEOLImages []*SiloImage
			for _, image := range c.SiloImages.Arm64 {
				if image.IsEOL == false {
					nonEOLImages = append(nonEOLImages, image)
				}
			}
			return nonEOLImages
		}
		return c.SiloImages.Arm64
	case common.Amd64:
		if !includeEOL {
			var nonEOLImages []*SiloImage
			for _, image := range c.SiloImages.Amd64 {
				if image.IsEOL == false {
					nonEOLImages = append(nonEOLImages, image)
				}
			}
			return nonEOLImages
		}
		return c.SiloImages.Amd64
	default:
		return nil
	}
}

func (c *SiloImageCatalogue) GetImageByID(archType common.ArchType, id string) *SiloImage {
	images := c.ListImages(archType, true) //where a specific image is requested by id, we want to include EOL images in the search, as they may still be referenced on a user's system.
	for _, image := range images {
		if image.ReleaseVersion == id {
			return image
		}
	}
	return nil
}

func (c *SiloImageCatalogue) GetImageByDistroAndVersion(archType common.ArchType, distro string, version string) []*SiloImage {

	//call into our public interface method to get all images for the specified architecture.
	images := c.ListImages(archType, false)

	//crucially, multiple images may exist for the same distro and version, but with different release versions.
	//thus we store all matches to our array..
	var matchingImages []*SiloImage
	for _, image := range images {
		if image.Distro == distro && image.Version == version {
			matchingImages = append(matchingImages, image)
		}
	}

	return matchingImages
}

// SiloImage represents a single Silo image in the catalogue.
type SiloImage struct {
	Distro          string `json:"distro"`
	Version         string `json:"version"`
	ReleaseVersion  string `json:"release_version"`
	ReleaseChecksum string `json:"release_checksum"`
	Description     string `json:"description"`
	ImageURL        string `json:"image_url"`
	ImageSize       string `json:"image_size"`
	IsEOL           bool   `json:"is_eol"`
	CatalogueName   string `json:"catalogue_name"`
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
	Arm64 []*SiloImage `json:"arm64"`
	Amd64 []*SiloImage `json:"amd64"`
}
