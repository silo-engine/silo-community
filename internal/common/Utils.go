package common

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
)

//ComputeChecksum computes the SHA256 checksum of a file at the given path.
func ComputeChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to compute checksum: %w", err)
	}

	return hex.EncodeToString(hash.Sum(nil)), nil

}


// opyFile copies a file from src to dst. If dst does not exist, it will be created. If it does exist, it will be overwritten.
func CopyFile(src, dst string) error {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return err
	}

	if !sourceFileStat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", src)
	}

	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer source.Close()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destination.Close()

	_, err = io.Copy(destination, source)
	return err
}





// GetSystemArchType returns the architecture type of the current system.
func GetSystemArchType() (ArchType,error) {
	arch := runtime.GOARCH
	switch arch {
	case "arm64":
		return Arm64,nil
	case "amd64":
		return Amd64,nil
	default:
		return "", errors.New("unsupported architecture: " + arch)
	}
}