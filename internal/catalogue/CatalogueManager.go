package catalogue

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const IMAGE_CATALOGUES_BASE_PATH = "res/silo-images/catalogues"
const OFFICIAL_BASE_IMAGE_CATALOGUE_FILE_NAME = "silo-image-catalogue.json"

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

//helpers

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
