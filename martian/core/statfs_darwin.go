//go:build darwin
// +build darwin

package core

//
// File system query utility for Darwin.
//

import (
	"fmt"
	"os"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func FsTypeString(fsType int64) string {
	switch fsType {
	case 17:
		return "hfs"
	case 24:
		return "apfs"
	case 25:
		return "devfs"
	case 26:
		return "autofs"
	case 27:
		return "zfs"
	default:
		return fmt.Sprintf("unknown (%#x)", fsType)
	}
}

func GetAvailableSpace(path string) (bytes, inodes uint64, fstype string, err error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return 0, 0, "", err
	}
	return fs.Bavail * uint64(fs.Bsize), fs.Ffree, statfsTypeName(&fs), nil
}

// The minimum number of inodes available in the pipestance directory
// below which the pipestance will not run.
const PIPESTANCE_MIN_INODES uint64 = 500

// The minimum amount of available disk space for a pipestance directory.
// If the available space falls below this at any time during the run, the
// the pipestance is killed.
const PIPESTANCE_MIN_DISK uint64 = 50 * 1024 * 1024

var disableDiskSpaceCheck = (os.Getenv("MRO_DISK_SPACE_CHECK") == disable)

// Returns an error if the current available space on the disk drive is
// very low.
func CheckMinimalSpace(path string) error {
	if disableDiskSpaceCheck {
		return nil
	}
	bytes, inodes, _, err := GetAvailableSpace(path)
	if err != nil {
		return err
	}
	// Allow zero, as if we haven't already failed to write a file it's
	// likely that the filesystem is just lying to us.
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
	if inodes < PIPESTANCE_MIN_INODES && inodes != 0 {
		return &DiskSpaceError{
			Bytes:  bytes,
			Inodes: inodes,
			Message: fmt.Sprintf(
				`%s has only %d free inodes remaining.
To ignore this error, set MRO_DISK_SPACE_CHECK=disable in your environment.`,
				path, inodes),
		}
	}
	return nil
}

// GetMountOptions returns the mount type and best-effort mount options for the
// mount on which the given path exists.
func GetMountOptions(path string) (fstype, opts string, err error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(path, &fs); err != nil {
		return "", "", err
	}
	return statfsTypeName(&fs), darwinMountOptions(fs.Flags), nil
}

func statfsTypeName(fs *syscall.Statfs_t) string {
	var builder strings.Builder
	for _, c := range fs.Fstypename {
		if c == 0 {
			break
		}
		builder.WriteByte(byte(c))
	}
	if builder.Len() > 0 {
		return builder.String()
	}
	return FsTypeString(int64(fs.Type))
}

func darwinMountOptions(flags uint32) string {
	opts := make([]string, 0, 8)
	if flags&unix.MNT_RDONLY != 0 {
		opts = append(opts, "ro")
	} else {
		opts = append(opts, "rw")
	}
	if flags&unix.MNT_LOCAL != 0 {
		opts = append(opts, "local")
	} else {
		opts = append(opts, "remote")
	}
	if flags&unix.MNT_SYNCHRONOUS != 0 {
		opts = append(opts, "sync")
	}
	if flags&unix.MNT_ASYNC != 0 {
		opts = append(opts, "async")
	}
	if flags&unix.MNT_NOEXEC != 0 {
		opts = append(opts, "noexec")
	}
	if flags&unix.MNT_NOSUID != 0 {
		opts = append(opts, "nosuid")
	}
	if flags&unix.MNT_NODEV != 0 {
		opts = append(opts, "nodev")
	}
	return strings.Join(opts, ",")
}
