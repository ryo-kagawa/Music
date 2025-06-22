package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"strconv"

	"github.com/ryo-kagawa/Music/types/cdda"
	"github.com/ryo-kagawa/go-utils/commandline"
	"github.com/ryo-kagawa/go-utils/conditional"
	"golang.org/x/sys/windows"
)

// Channels * Bit Depth
const sampleSize = 2 * 16 / 8

type RAW_READ_INFO struct {
	DiskOffset  int64
	SectorCount uint32
	TrackMode   uint32
}

func msfToLBA(min, sec, frame byte) int {
	return (((int(min) * 60) + int(sec)) * 75) + int(frame)
}

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	// Win32 Device Namespaces
	win32DeviceNamespaces := arguments[0]
	win32DeviceNamespacesPtr, err := windows.UTF16PtrFromString(win32DeviceNamespaces)
	if err != nil {
		return "", err
	}
	verifyCount, err := strconv.Atoi(arguments[1])
	if err != nil {
		return "", err
	}
	offsetSample, err := strconv.Atoi(arguments[2])
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		win32DeviceNamespacesPtr,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	toc, err := cdda.ReadTOC(handle)
	if err != nil {
		return "", err
	}

	tocLength := binary.BigEndian.Uint16(toc.Length[0:2]) - 2
	leadOutLBA := 0
	track01LBA := 0

	for i := range tocLength / 11 {
		descriptor := toc.Descriptors[i]
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
	result, err := cdda.ReadAllSector(handle, leadOutLBA-track01LBA, verifyCount)
	if err != nil {
		return "", nil
	}

	if offsetSample < 0 {
		offset := -offsetSample * sampleSize
		result = append(bytes.Repeat([]byte{0x00}, offset), result[:len(result)-offset]...)
	}
	if 0 < offsetSample {
		offset := offsetSample * sampleSize
		result = append(result[offset:], bytes.Repeat([]byte{0x00}, offset)...)
	}

	outFile, err := os.Create("file.wav")
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	header := []byte("RIFF")
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(len(result)+36))...)
	header = append(header, []byte("WAVE")...)
	header = append(header, []byte("fmt ")...)
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(16))...)
	header = append(header, binary.LittleEndian.AppendUint16([]byte{}, uint16(1))...)
	header = append(header, binary.LittleEndian.AppendUint16([]byte{}, uint16(2))...)
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(44100))...)
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(44100*2*16/8))...)
	header = append(header, binary.LittleEndian.AppendUint16([]byte{}, uint16(2*16/8))...)
	header = append(header, binary.LittleEndian.AppendUint16([]byte{}, uint16(16))...)

	// dataチャンク
	header = append(header, []byte("data")...)
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(len(result)))...)
	_, err = outFile.Write(header)
	if err != nil {
		return "", err
	}
	_, err = outFile.Write(result)
	if err != nil {
		return "", err
	}

	return "finish", nil
}
