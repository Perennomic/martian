//go:build windows
// +build windows

package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

func FsTypeString(int64) string {
	return ""
}

func GetAvailableSpace(path string) (bytes, inodes uint64, fstype string, err error) {
	if path == "" {
		path = "."
	}
	path = windowsDirectoryPath(path)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, 0, "", err
	}
	var freeBytesAvailableToCaller uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64
	if err := windows.GetDiskFreeSpaceEx(
		pathPtr,
		&freeBytesAvailableToCaller,
		&totalNumberOfBytes,
		&totalNumberOfFreeBytes); err != nil {
		return 0, 0, "", err
	}
	fstype, _, err = getWindowsVolumeInformation(path)
	if err != nil {
		return 0, 0, "", err
	}
	return freeBytesAvailableToCaller, 0, fstype, nil
}

// The minimum amount of available disk space for a pipestance directory.
// If the available space falls below this at any time during the run, the
// the pipestance is killed.
const PIPESTANCE_MIN_DISK uint64 = 50 * 1024 * 1024

var disableDiskSpaceCheck = (os.Getenv("MRO_DISK_SPACE_CHECK") == disable)

// Returns an error if the current available space on the disk drive is
// very low. Windows does not expose Unix-style inode availability through
// GetDiskFreeSpaceEx, so inode checks are intentionally skipped.
func CheckMinimalSpace(path string) error {
	if disableDiskSpaceCheck {
		return nil
	}
	bytes, inodes, _, err := GetAvailableSpace(path)
	if err != nil {
		return err
	}
	if bytes < PIPESTANCE_MIN_DISK && bytes != 0 {
		return &DiskSpaceError{
			Bytes:  bytes,
			Inodes: inodes,
			Message: fmt.Sprintf(
				`%s has only %dkB remaining space available.
To ignore this error, set MRO_DISK_SPACE_CHECK=disable in your environment.`,
				path, bytes/1024),
		}
	}
	return nil
}

// GetMountOptions returns the Windows filesystem type and best-effort volume
// capabilities for the volume on which the given path exists.
func GetMountOptions(path string) (fstype, opts string, err error) {
	fstype, flags, err := getWindowsVolumeInformation(path)
	if err != nil {
		return "", "", err
	}
	volumePath, err := windowsVolumePath(path)
	if err != nil {
		return "", "", err
	}
	return fstype, windowsVolumeOptions(volumePath, flags), nil
}

func getWindowsVolumeInformation(path string) (fstype string, flags uint32, err error) {
	path = windowsDirectoryPath(path)
	volumePath, err := windowsVolumePath(path)
	if err != nil {
		return "", 0, err
	}
	volumePtr, err := windows.UTF16PtrFromString(volumePath)
	if err != nil {
		return "", 0, err
	}
	var volumeName [windows.MAX_PATH + 1]uint16
	var filesystemName [windows.MAX_PATH + 1]uint16
	var serialNumber uint32
	var maximumComponentLength uint32
	if err := windows.GetVolumeInformation(
		volumePtr,
		&volumeName[0],
		uint32(len(volumeName)),
		&serialNumber,
		&maximumComponentLength,
		&flags,
		&filesystemName[0],
		uint32(len(filesystemName))); err != nil {
		return "", 0, err
	}
	return windows.UTF16ToString(filesystemName[:]), flags, nil
}

func windowsVolumePath(path string) (string, error) {
	if path == "" {
		path = "."
	}
	path = windowsDirectoryPath(path)
	pathPtr, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	var volumePath [windows.MAX_PATH + 1]uint16
	if err := windows.GetVolumePathName(pathPtr, &volumePath[0], uint32(len(volumePath))); err != nil {
		return "", err
	}
	return windows.UTF16ToString(volumePath[:]), nil
}

func windowsDirectoryPath(path string) string {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return filepath.Dir(path)
	}
	return path
}

func windowsVolumeOptions(volumePath string, flags uint32) string {
	opts := make([]string, 0, 12)
	if flags&windows.FILE_READ_ONLY_VOLUME != 0 {
		opts = append(opts, "ro")
	} else {
		opts = append(opts, "rw")
	}
	if driveType := windowsDriveType(volumePath); driveType != "" {
		opts = append(opts, driveType)
	}
	if flags&windows.FILE_SUPPORTS_HARD_LINKS != 0 {
		opts = append(opts, "hardlinks")
	}
	if flags&windows.FILE_SUPPORTS_REPARSE_POINTS != 0 {
		opts = append(opts, "reparse_points")
	}
	if flags&windows.FILE_SUPPORTS_SPARSE_FILES != 0 {
		opts = append(opts, "sparse_files")
	}
	if flags&windows.FILE_SUPPORTS_ENCRYPTION != 0 {
		opts = append(opts, "encryption")
	}
	if flags&windows.FILE_SUPPORTS_EXTENDED_ATTRIBUTES != 0 {
		opts = append(opts, "extended_attributes")
	}
	if flags&windows.FILE_VOLUME_QUOTAS != 0 {
		opts = append(opts, "quotas")
	}
	if flags&windows.FILE_VOLUME_IS_COMPRESSED != 0 {
		opts = append(opts, "compressed")
	}
	if flags&windows.FILE_CASE_SENSITIVE_SEARCH != 0 {
		opts = append(opts, "case_sensitive_search")
	}
	return strings.Join(opts, ",")
}

func windowsDriveType(volumePath string) string {
	volumePtr, err := windows.UTF16PtrFromString(volumePath)
	if err != nil {
		return ""
	}
	switch windows.GetDriveType(volumePtr) {
	case windows.DRIVE_REMOVABLE:
		return "removable"
	case windows.DRIVE_FIXED:
		return "fixed"
	case windows.DRIVE_REMOTE:
		return "remote"
	case windows.DRIVE_CDROM:
		return "cdrom"
	case windows.DRIVE_RAMDISK:
		return "ramdisk"
	default:
		return ""
	}
}
