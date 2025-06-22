package cdda

import (
	"bytes"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	DISK_OFFSET_SIZE     = 2048
	IOCTL_CDROM_RAW_READ = 0x0002403E
	RAW_SECTOR_SIZE      = 2352
	TRACK_MODE_TYPE_CDDA = 2
)

type RAW_READ_INFO struct {
	DiskOffset  int64
	SectorCount uint32
	TrackMode   uint32
}

func ReadAllSector(handle windows.Handle, endLBA int, verifyCount int) ([]byte, error) {
	result := make([]byte, 0, endLBA*RAW_SECTOR_SIZE)
	for lba := range endLBA {
		sectorBuffer, err := ReadSector(handle, lba, verifyCount)
		if err != nil {
			return nil, fmt.Errorf("lba: %d not read: %v", lba, err)
		}
		result = append(result, sectorBuffer...)
	}

	return result, nil
}

func readSector(handle windows.Handle, sector int) ([]byte, error) {
	rawInfo := RAW_READ_INFO{
		DiskOffset:  int64(sector * DISK_OFFSET_SIZE),
		SectorCount: 1,
		TrackMode:   TRACK_MODE_TYPE_CDDA,
	}
	sectorBuffer := make([]byte, RAW_SECTOR_SIZE)
	readBytes := uint32(0)
	if err := windows.DeviceIoControl(
		handle,
		IOCTL_CDROM_RAW_READ,
		(*byte)(unsafe.Pointer(&rawInfo)),
		uint32(unsafe.Sizeof(rawInfo)),
		&sectorBuffer[0],
		uint32(len(sectorBuffer)),
		&readBytes,
		nil,
	); err != nil {
		return nil, fmt.Errorf("sector: %d not read: %v", sector, err)
	}

	return sectorBuffer, nil
}

func ReadSector(handle windows.Handle, sector int, verifyCount int) ([]byte, error) {
	sectorBinary, err := readSector(handle, sector)
	if err != nil {
		return nil, err
	}

	for range verifyCount {
		verify, err := readSector(handle, sector)
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(sectorBinary, verify) {
			return nil, fmt.Errorf("sector: %d not verify", sector)
		}
	}

	return sectorBinary, nil
}
