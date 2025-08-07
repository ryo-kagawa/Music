package cdda

import (
	"encoding/binary"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_AUDIO_WITH_PREEMPHASIS = 0x1
	CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_DIGITAL_COPY_PERMITTED = 0x2
	CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_AUDIO_DATA_TRACK       = 0x4
	CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_TWO_FOUR_CHANNEL_AUDIO = 0x8
)
const IOCTL_CDROM_READ_TOC_EX = 0x00024054

type CDROM_READ_TOC_EX struct {
	// 0-3: Format
	// 4-6: Reserved1
	// 7: Msf
	Format_Reserved1_Msf byte
	SessionTrack         byte
	Reserved2            byte
	Reserved3            byte
}

type CDROM_TOC_FULL_TOC_DATA_BLOCK struct {
	SessionNumber byte
	// 0-3: Control
	// 4-7: Adr
	Control_Adr byte
	Reserved1   byte
	Point       byte
	MsfExtra    [3]byte
	Zero        byte
	Msf         [3]byte
}

func (c CDROM_TOC_FULL_TOC_DATA_BLOCK) GetAdr() byte {
	return c.Control_Adr >> 4
}
func (c CDROM_TOC_FULL_TOC_DATA_BLOCK) GetControl() byte {
	return c.Control_Adr & 0xF
}
func (c CDROM_TOC_FULL_TOC_DATA_BLOCK) HasAudioWithPreEmphasis() bool {
	return c.GetControl()&CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_AUDIO_WITH_PREEMPHASIS != 0
}
func (c CDROM_TOC_FULL_TOC_DATA_BLOCK) HasDigitalCopyPermited() bool {
	return c.GetControl()&CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_DIGITAL_COPY_PERMITTED != 0
}
func (c CDROM_TOC_FULL_TOC_DATA_BLOCK) HasAudioDataTrack() bool {
	return c.GetControl()&CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_AUDIO_DATA_TRACK != 0
}
func (c CDROM_TOC_FULL_TOC_DATA_BLOCK) HasTwoFourChannelAudio() bool {
	return c.GetControl()&CDROM_TOC_FULL_TOC_DATA_BLOCK_CONTROL_TWO_FOUR_CHANNEL_AUDIO != 0
}

type CDROM_TOC_FULL_TOC_DATA struct {
	Length               [2]byte
	FirstCompleteSession byte
	LastCompleteSession  byte
	Descriptors          []CDROM_TOC_FULL_TOC_DATA_BLOCK
}

func readTOC(handle windows.Handle, bufferSize int) ([]byte, error) {
	input := CDROM_READ_TOC_EX{
		Format_Reserved1_Msf: byte(0x02 | (1 << 7)),
		SessionTrack:         byte(0),
		Reserved2:            byte(0),
		Reserved3:            byte(0),
	}
	buffer := make([]byte, bufferSize)
	if err := windows.DeviceIoControl(
		handle,
		IOCTL_CDROM_READ_TOC_EX,
		(*byte)(unsafe.Pointer(&input)),
		uint32(unsafe.Sizeof(input)),
		&buffer[0],
		uint32(len(buffer)),
		new(uint32),
		nil,
	); err != nil {
		return nil, err
	}
	length := int(binary.BigEndian.Uint16(buffer[0:2])) + 2
	if bufferSize < length {
		return readTOC(handle, length)
	}
	return buffer, nil
}

func ReadTOC(handle windows.Handle) (CDROM_TOC_FULL_TOC_DATA, error) {
	buffer, err := readTOC(handle, 2048)
	if err != nil {
		return CDROM_TOC_FULL_TOC_DATA{}, err
	}

	toc := CDROM_TOC_FULL_TOC_DATA{
		Length:               [2]byte(buffer[0:2]),
		FirstCompleteSession: buffer[2],
		LastCompleteSession:  buffer[3],
	}
	for i := range int(binary.BigEndian.Uint16(toc.Length[0:2])-2) / 11 {
		offset := 4 + i*11
		descriptor := CDROM_TOC_FULL_TOC_DATA_BLOCK{
			SessionNumber: buffer[offset],
			Control_Adr:   buffer[offset+1],
			Reserved1:     buffer[offset+2],
			Point:         buffer[offset+3],
			MsfExtra:      [3]byte{buffer[offset+4], buffer[offset+5], buffer[offset+6]},
			Zero:          buffer[offset+7],
			Msf:           [3]byte{buffer[offset+8], buffer[offset+9], buffer[offset+10]},
		}
		toc.Descriptors = append(toc.Descriptors, descriptor)
	}

	return toc, nil
}
