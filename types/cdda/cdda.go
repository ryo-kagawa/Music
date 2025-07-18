package cdda

import (
	"fmt"
	"unsafe"

	"github.com/ryo-kagawa/go-utils/conditional"
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

func msfToLBA(min, sec, frame byte) int {
	return (((int(min) * 60) + int(sec)) * 75) + int(frame)
}

func ReadAllSector(handle windows.Handle) ([]byte, error) {
	toc, err := ReadTOC(handle)
	if err != nil {
		return nil, err
	}

	leadOutLBA := 0
	track01LBA := 0
	for _, descriptor := range toc.Descriptors {
		switch descriptor.Point {
		case 0x00:
		case 0x01:
			track01LBA = conditional.Value(
				msfToLBA(descriptor.MsfExtra[0], descriptor.MsfExtra[1], descriptor.MsfExtra[2]) != 0,
				msfToLBA(descriptor.MsfExtra[0], descriptor.MsfExtra[1], descriptor.MsfExtra[2]),
				msfToLBA(descriptor.Msf[0], descriptor.Msf[1], descriptor.Msf[2]),
			)
		case 0xA0:
		case 0xA1:
		case 0xA2:
			leadOutLBA = conditional.Value(
				msfToLBA(descriptor.MsfExtra[0], descriptor.MsfExtra[1], descriptor.MsfExtra[2]) != 0,
				msfToLBA(descriptor.MsfExtra[0], descriptor.MsfExtra[1], descriptor.MsfExtra[2]),
				msfToLBA(descriptor.Msf[0], descriptor.Msf[1], descriptor.Msf[2]),
			)
		case 0xB0:
		case 0xB1:
		}
	}
	endLBA := leadOutLBA - track01LBA

	result := make([]byte, 0, endLBA*RAW_SECTOR_SIZE)
	for lba := range endLBA {
		sectorBuffer, err := ReadSector(handle, lba)
		if err != nil {
			return nil, fmt.Errorf("lba: %d not read: %v", lba, err)
		}
		result = append(result, sectorBuffer...)
	}

	return result, nil
}

func ReadSector(handle windows.Handle, sector int) ([]byte, error) {
	rawInfo := RAW_READ_INFO{
		DiskOffset:  int64(sector * DISK_OFFSET_SIZE),
		SectorCount: 1,
		TrackMode:   TRACK_MODE_TYPE_CDDA,
	}
	sectorBuffer := make([]byte, RAW_SECTOR_SIZE)
	if err := windows.DeviceIoControl(
		handle,
		IOCTL_CDROM_RAW_READ,
		(*byte)(unsafe.Pointer(&rawInfo)),
		uint32(unsafe.Sizeof(rawInfo)),
		&sectorBuffer[0],
		uint32(len(sectorBuffer)),
		new(uint32),
		nil,
	); err != nil {
		return nil, fmt.Errorf("sector: %d not read: %v", sector, err)
	}

	return sectorBuffer, nil
}
