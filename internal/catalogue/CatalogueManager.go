package catalogue

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const IMAGE_CATALOGUES_BASE_PATH = "res/silo-images/catalogues"
const OFFICIAL_BASE_IMAGE_CATALOGUE_FILE_NAME = "silo-image-catalogue.json"
const IMAGE_INSTALLATION_BASE_PATH = "res/silo-images"
const IMAGE_INSTALLATION_BASE_PATH_ARM64 = IMAGE_INSTALLATION_BASE_PATH + "/arm64"
const IMAGE_INSTALLATION_BASE_PATH_AMD64 = IMAGE_INSTALLATION_BASE_PATH + "/amd64"

// CatalogueManager manages the silo image catalogues.
type CatalogueManager struct {

	// catalogues is a map of catalogues, keyed by their name.
	catalogues map[string]*SiloImageCatalogue
}

// NewCatalogueManager creates a new CatalogueManager.
func NewCatalogueManager() *CatalogueManager {

	//instansiate a new CatalogueManager with an empty map of catalogues
	cat := &CatalogueManager{
		catalogues: make(map[string]*SiloImageCatalogue),
	}

	//load all catalogues from JSON files in the IMAGE_CATALOGUES_BASE_PATH directory
	err := cat.loadAllCataloguesFromJSONFiles()
	if err != nil {
		e := errors.New("failed to load catalogues from JSON files: " + err.Error())
		panic(e)
	}

	return cat
}

// GetBaseImageCatalogue retrieves the official base image catalogue, which is expected to be named "silo-base-image-catalogue".
func (cm *CatalogueManager) GetBaseImageCatalogue() (*SiloImageCatalogue, bool) {
	return cm.GetCatalogue("silo-base-image-catalogue") //the "name" field in the JSON file is "silo-base-image-catalogue" and this is the key we use to retrieve it from the map.
}

// GetBaseImageCatalogueDefaultImage retrieves the default Silo image for the official base image catalogue for a given architecture type.
func (cm *CatalogueManager) GetBaseImageCatalogueDefaultImage(archType ArchType) (*SiloImage, error) {
	catalogue, exists := cm.GetBaseImageCatalogue()
	if !exists {
		return nil, errors.New("base image catalogue not found")
	}

	defaultImage := catalogue.GetDefaultImage(archType)
	if defaultImage == nil {
		return nil, errors.New("default image not found for architecture: " + string(archType))
	}

	return defaultImage, nil
}

// List BaseImageCatalogueImages lists all Silo images for the official base image catalogue for a given architecture type.
func (cm *CatalogueManager) ListBaseImageCatalogueImages(archType ArchType) ([]SiloImage, error) {
	catalogue, exists := cm.GetBaseImageCatalogue()
	if !exists {
		return nil, errors.New("base image catalogue not found")
	}

	images := catalogue.ListImages(archType)
	if images == nil {
		return nil, errors.New("no images found for architecture: " + string(archType))
	}

	return images, nil
}

// GetCatalogue retrieves a SiloImageCatalogue by name.
func (cm *CatalogueManager) GetCatalogue(name string) (*SiloImageCatalogue, bool) {
	catalogue, exists := cm.catalogues[name]
	return catalogue, exists
}

// List all catalogue by their name (i.e in the name field in the JSON file, which is also the key we use to retrieve it from the map).
func (cm *CatalogueManager) ListCatalogueNames() []string {
	catalogueNames := make([]string, 0, len(cm.catalogues))

	//for each catalogue in the map, append its key (i.e. its name) to the list of catalogue names
	for name := range cm.catalogues {
		catalogueNames = append(catalogueNames, name)
	}

	return catalogueNames
}

// List all Catalogues (i.e. the SiloImageCatalogue structs) in a slice.
func (cm *CatalogueManager) ListCatalogues() []*SiloImageCatalogue {
	catalogues := make([]*SiloImageCatalogue, 0, len(cm.catalogues))

	//for each catalogue in the map, append its value (i.e. the SiloImageCatalogue struct) to the list of catalogues
	for _, catalogue := range cm.catalogues {
		catalogues = append(catalogues, catalogue)
	}

	return catalogues
}

// ListCatalogueIMages lists all Silo images for a given catalogue name and architecture type.
func (cm *CatalogueManager) ListCatalogueImages(catalogueName string, archType ArchType) ([]SiloImage, error) {
	catalogue, exists := cm.GetCatalogue(catalogueName)
	if !exists {
		return nil, errors.New("catalogue not found: " + catalogueName)
	}

	images := catalogue.ListImages(archType)
	if images == nil {
		return nil, errors.New("no images found for architecture: " + string(archType))
	}

	return images, nil
}

// DownloadCatalogueImage downloads a specific Silo image from a given catalogue name, architecture type, and image name to a specified destination path.
func (cm *CatalogueManager) DownloadCatalogueImage(catalogueName string, imageIdOrName string) error {

	if catalogueName == "" {
		fmt.Println("silo: No catalogue image provided defaulting to the official base image catalogue: silo-base-image-catalogue")
		catalogueName = "silo-base-image-catalogue"
	}

	if imageIdOrName == "" {
		return errors.New("no image id or name provided")
	}

	catalogue, exists := cm.GetCatalogue(catalogueName)
	if !exists {
		return errors.New("catalogue not found: " + catalogueName)
	}

	archType, err := GetSystemArchType()
	if err != nil {
		return errors.New("failed to get system architecture type: " + err.Error())
	}

	// Note: image ids are ALWAYS numeric and images names are are always alphanumeric with a hyphen in the middle,
	// e.g. ubuntu-22.04 or debian-12.0 thus we may determine whether the user has specified an image id or an image name by
	// either checking if the string is numeric or checking if the string contains a hyphen. We elect to check for the latter
	// as its less likely to result in a false positive where a user decides to name an image with a numeric string,
	// which is not recommended but possible.
	if strings.Contains(imageIdOrName, "-") {
		//imageIdOrName is an image name, so we will download the image by name.
		return cm.downloadCatalogueImageByName(catalogue, archType, imageIdOrName)
	} else {
		//imageIdOrName is an image id, so we will download the image by id.
		return cm.downloadCatalogueImageById(catalogue, archType, imageIdOrName)
	}

}

//helpers

func (cm *CatalogueManager) downloadCatalogueImageById(catalogue *SiloImageCatalogue, archType ArchType, imageId string) error {

	image := catalogue.GetImageByID(archType, imageId)
	if image == nil {
		return errors.New("image not found: " + imageId + " for architecture: " + string(archType))
	}

	if err := cm.downloadAndUnpackCatalogueImage(image, archType); err != nil {
		return errors.New("failed to download and unpack image: " + err.Error())
	}
	return nil

}

// downloadCatalogueImageByName downloads a specific Silo image from a given catalogue name and image name.
// the image name is merely a concatenation of the distro name and version number, e.g. "ubuntu-22.04" or "debian-12.0"
// and is not unique across catalogues or architectures although silo will only ever download the image for the current
//
//	system architecture type, where available. If the catalogue or image does not exist, an error is returned.
func (cm *CatalogueManager) downloadCatalogueImageByName(catalogue *SiloImageCatalogue, archType ArchType, imageName string) error {

	imageNameParts := strings.SplitN(imageName, "-", 2)
	if len(imageNameParts) != 2 {
		return errors.New("invalid image name: " + imageName + " expected format: <distro>-<version> e.g. ubuntu-22.04 or debian-12.0")
	}
	distro := imageNameParts[0]
	version := imageNameParts[1]
	image := catalogue.GetImageByDistroAndVersion(archType, distro, version)
	if image == nil {
		return errors.New("image not found: " + imageName + " for architecture: " + string(archType))
	}

	if err := cm.downloadAndUnpackCatalogueImage(image, archType); err != nil {
		return errors.New("failed to download and unpack image: " + err.Error())
	}
	return nil

}

func (cm *CatalogueManager) loadCatalogueFromJSONFile(filePath string) (*SiloImageCatalogue, error) {

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var catalogue SiloImageCatalogue
	err = json.Unmarshal(data, &catalogue)
	if err != nil {
		return nil, err
	}

	return &catalogue, nil
}

// loadAllCatalogueFromJSONFiles loads all SiloImageCatalogues from JSON files in the IMAGE_CATALOGUES_BASE_PATH directory.
// and returns a map of catalogues keyed by their name.
func (cm *CatalogueManager) loadAllCataloguesFromJSONFiles() error {

	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	catalogueDir := filepath.Join(filepath.Dir(exe), IMAGE_CATALOGUES_BASE_PATH)

	files, err := os.ReadDir(catalogueDir)
	if err != nil {
		return err
	}

	for _, file := range files {

		//skip dirs
		if file.IsDir() {
			continue
		}

		//skip non JSON files
		if file.Type().IsRegular() && file.Name()[len(file.Name())-5:] != ".json" {
			continue
		}

		//load/parse the catalogue from the JSON file
		catalogue, err := cm.loadCatalogueFromJSONFile(IMAGE_CATALOGUES_BASE_PATH + "/" + file.Name())
		if err != nil {
			return err
		}
		cm.catalogues[catalogue.Name] = catalogue

	}

	return nil
}

// downloadAndUnpackCatalogueImage downloads a specific Silo image from a given catalogue name, architecture type, and image name to a specified destination path.
func (cm *CatalogueManager) downloadAndUnpackCatalogueImage(image *SiloImage, archType ArchType) error {

	fmt.Println("\nDownloading Silo image:", image.Distro, image.Version, "for architecture:", archType, "standby this may take a few minutes depending on your internet connection...")
	imageURL := image.ImageURL

	//download the image from the URL
	resp, err := http.Get(imageURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	//check if the response status code is 200 OK
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download image: %s, status code: %d", imageURL, resp.StatusCode)
	}
	

	//create the destination path for the downloaded image
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	//prepare the destination path for the downloaded image based on its architecture type and name
	var imageInstallationBasePath string
	if archType == Arm64 {
		imageInstallationBasePath = IMAGE_INSTALLATION_BASE_PATH_ARM64
	} else if archType == Amd64 {
		imageInstallationBasePath = IMAGE_INSTALLATION_BASE_PATH_AMD64
	} else {
		return fmt.Errorf("unsupported architecture type: %s", archType)
	}
	imageRootFolderName := image.Distro + "-" + image.Version + "--" + string(archType) + "-release"
	destPath := filepath.Join(filepath.Dir(exe),
		imageInstallationBasePath,
		imageRootFolderName)

	//create the destination directory if it doesn't exist
	err = os.MkdirAll(destPath, 0755)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	//unpack the downloaded image to the destination path
	err = extractBaseImage(resp, destPath)
	if err != nil {
		return fmt.Errorf("failed to unpack image: %w", err)
	}

	//finally write the checksum of the downloaded base image to the release directory for reference.
	// under the file checksum.chk This will later be used by the SiloEngine to ensure exported Sessions
	// can always be reconstructed with exactly the same base image that was used to create them, even if
	// the base image is updated in the future.
	checksumFilePath := filepath.Join(destPath, "checksum.chk")
	err = os.WriteFile(checksumFilePath, []byte(image.ReleaseChecksum+"\n"), 0644)
	if err != nil {
		return fmt.Errorf("failed to write checksum to release directory: %w", err)
	}

	fmt.Println("Successfully downloaded and unpacked Silo image:", image.Distro, image.Version, "for architecture:", archType)

	return nil

}

func extractBaseImage(resp *http.Response, destDir string) error {
	defer resp.Body.Close()

	cmd := exec.Command(
		"tar",
		"-xzf", "-",
		"-C", destDir,
	)

	cmd.Stdin = resp.Body

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract base image: %w: %s", err, output)
	}

	return nil
}
