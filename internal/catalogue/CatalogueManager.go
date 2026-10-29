package catalogue

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	catalogues     map[string]*SiloImageCatalogue
	systemArchType ArchType
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

	//set the system CPU architecture type, this will be used to determine which images apply to the current system
	//currently only arm64 and amd64 are supported, but this may be extended in the future to support other architectures.
	archType, err := GetSystemArchType()
	if err != nil {
		e := errors.New("failed to get system architecture type: " + err.Error())
		panic(e)
	}

	//check that the system architecture type is supported, if not panic as this is a fatal error.
	if archType != Arm64 && archType != Amd64 {
		e := errors.New("unsupported system architecture type: " + string(archType) + " only arm64 and amd64 are supported")
		panic(e)
	}

	cat.systemArchType = archType

	img, _ := cat.GetCatalogueImage("ubuntu-25.04")
	fmt.Println(img.IsInstalled())

	return cat
}

// GetBaseImageCatalogue retrieves the official base image catalogue, which is expected to be named "silo-base-image-catalogue".
func (cm *CatalogueManager) GetBaseImageCatalogue() (*SiloImageCatalogue, bool) {
	return cm.GetCatalogue("silo-base-image-catalogue") //the "name" field in the JSON file is "silo-base-image-catalogue" and this is the key we use to retrieve it from the map.
}

// GetBaseImageCatalogueDefaultImage retrieves the default Silo image for the official base image catalogue for a given architecture type.
func (cm *CatalogueManager) GetBaseImageCatalogueDefaultImage() (*SiloImage, error) {
	catalogue, exists := cm.GetBaseImageCatalogue()
	if !exists {
		return nil, errors.New("base image catalogue not found")
	}

	defaultImage := catalogue.GetDefaultImage(cm.systemArchType)
	defaultImage.SetArchString(string(cm.systemArchType))
	if defaultImage == nil {
		return nil, errors.New("default image not found for architecture: " + string(cm.systemArchType))
	}

	return defaultImage, nil
}

// List BaseImageCatalogueImages lists all Silo images for the official base image catalogue for a given architecture type.
func (cm *CatalogueManager) ListBaseImageCatalogueImages() ([]*SiloImage, error) {
	catalogue, exists := cm.GetBaseImageCatalogue()
	if !exists {
		return nil, errors.New("base image catalogue not found")
	}

	images := catalogue.ListImages(cm.systemArchType)
	if images == nil {
		return nil, errors.New("no images found for architecture: " + string(cm.systemArchType))
	}

	return images, nil
}

// GetCatalogue retrieves a SiloImageCatalogue by name.
func (cm *CatalogueManager) GetCatalogue(name string) (*SiloImageCatalogue, bool) {
	catalogue, exists := cm.catalogues[name]
	return catalogue, exists
}

func (cm *CatalogueManager) GetCatalogueImage(imageNameOrId string) (*SiloImage, bool) {

	if imageNameOrId == "" {
		return nil, false
	}

	for _, catalogue := range cm.catalogues {
		if imageReferencedByName(imageNameOrId) {
			//where image is specified by name split the name into its distro and version parts and search for the image by those values.
			imageNameOrIdParts := strings.SplitN(imageNameOrId, "-", 2)
			distro := imageNameOrIdParts[0]
			version := imageNameOrIdParts[1]
			if image := catalogue.GetImageByDistroAndVersion(cm.systemArchType, distro, version); image != nil {
				image.SetArchString(string(cm.systemArchType))
				return image, true
			}
		} else {
			//otherwise search for the image by its id, which is always numeric and unique across catalogues and architectures.
			if image := catalogue.GetImageByID(cm.systemArchType, imageNameOrId); image != nil {
				image.SetArchString(string(cm.systemArchType))
				return image, true
			}
		}
	}

	return nil, false
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
func (cm *CatalogueManager) ListCatalogueImages(catalogueName string) ([]*SiloImage, error) {
	catalogue, exists := cm.GetCatalogue(catalogueName)
	if !exists {
		return nil, errors.New("catalogue not found: " + catalogueName)
	}

	images := catalogue.ListImages(cm.systemArchType)
	if images == nil {
		return nil, errors.New("no images found for architecture: " + string(cm.systemArchType))
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
	if imageReferencedByName(imageIdOrName) {
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

		//mark any images that are already installed on the system as installed in the catalogue
		cm.markInstalledImages(catalogue)

		//set the architecture string for each image in the catalogue, this is used to display the architecture type in the UI.
		cm.setSiloImageArchString(catalogue)

	}

	return nil
}

// markInstalledImages checks if any images in the given catalogue are already installed on the system and marks them as installed in the catalogue.
func (cm *CatalogueManager) markInstalledImages(catalogue *SiloImageCatalogue) {
	for _, image := range catalogue.SiloImages.Arm64 {
		if cm.isImageInstalled(Arm64, image) {
			image.SetInstalled(true)
		}
	}
	for _, image := range catalogue.SiloImages.Amd64 {
		if cm.isImageInstalled(Amd64, image) {
			image.SetInstalled(true)
		}
	}
}

// SetSiloImageArchString sets the architecture string for each Silo image in the given catalogue
// based on its architecture type. This value is not persisted to disk.
func (cm *CatalogueManager) setSiloImageArchString(catalogue *SiloImageCatalogue) {
	for _, image := range catalogue.SiloImages.Arm64 {
		image.SetArchString("arm64")
	}
	for _, image := range catalogue.SiloImages.Amd64 {
		image.SetArchString("amd64")
	}
}

// isImageInstalled checks if a specific Silo image is currently installed by looking for
// the image's root folder in the IMAGE_INSTALLATION_BASE_PATH directory for the given architecture type.
func (cm *CatalogueManager) isImageInstalled(archType ArchType, image *SiloImage) bool {

	var imageInstallationBasePath string
	if archType == Arm64 {
		imageInstallationBasePath = IMAGE_INSTALLATION_BASE_PATH_ARM64
	} else if archType == Amd64 {
		imageInstallationBasePath = IMAGE_INSTALLATION_BASE_PATH_AMD64
	} else {
		return false
	}

	// build path to what the image's root release package folder would be, if it were installed, and check if it exists...
	// If it does, the image is considered installed.
	imageRootFolderName := image.Distro + "-" + image.Version + "--" + string(archType) + "-release"
	destPath := filepath.Join(imageInstallationBasePath, imageRootFolderName)
	if _, err := os.Stat(destPath); err == nil {
		return true
	}

	return false
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

	tmp, err := os.CreateTemp("", "silo-image-*.tar.gz")
	if err != nil {
		return fmt.Errorf("failed to create temporary image file: %w", err)
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()

	//before going any further, where the image has been listed with a checksum, as all official
	//Silo base images are, we will verify the checksum of the downloaded image against the
	// expected checksum in the catalogue to ensure that the image has not been tampered with
	// or corrupted during download.
	if image.ReleaseChecksum != "" {
		h := sha256.New()

		if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
			return fmt.Errorf("failed to download image: %w", err)
		}

		checksum := hex.EncodeToString(h.Sum(nil))
		if !strings.EqualFold(checksum, image.ReleaseChecksum) {
			return fmt.Errorf(
				"checksum mismatch: expected %s, got %s",
				image.ReleaseChecksum, checksum,
			)
		} else {
			fmt.Println("Successfully validated checksum of downloaded image:", image.Distro, image.Version, "for architecture:", archType)
		}
	} else {
		if _, err := io.Copy(tmp, resp.Body); err != nil {
			return fmt.Errorf("failed to download image: %w", err)
		}
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("failed to rewind temporary image: %w", err)
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
	err = extractBaseImage(tmp, destPath)
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

	//mark the image as installed in the (in memory catalogue
	image.SetInstalled(true)

	fmt.Println("Successfully downloaded and unpacked Silo image:", image.Distro, image.Version, "for architecture:", archType)

	return nil

}

func extractBaseImage(src io.Reader, destDir string) error {
	cmd := exec.Command(
		"tar",
		"-xzf", "-",
		"-C", destDir,
	)

	cmd.Stdin = src

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("extract base image: %w: %s", err, output)
	}

	return nil
}

func imageReferencedByName(imageNameOrId string) bool {
	//check if the imageNameOrId is a valid image name (i.e. contains a hyphen)
	if strings.Contains(imageNameOrId, "-") {
		return true
	}

	return false
}
