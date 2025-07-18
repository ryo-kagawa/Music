package main

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/ryo-kagawa/Music/types/cdda"
	"github.com/ryo-kagawa/go-utils/commandline"
	"golang.org/x/sys/windows"
)

// Channels * Bit Depth
const sampleSize = 2 * 16 / 8
const IOCTL_STORAGE_EJECT_MEDIA = 0x002D4808
const IOCTL_STORAGE_LOAD_MEDIA = 0x002D480C

type RAW_READ_INFO struct {
	DiskOffset  int64
	SectorCount uint32
	TrackMode   uint32
}

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	// Win32 Device Namespaces
	driveLetter := arguments[0]
	win32DeviceNamespaces := "\\\\.\\" + driveLetter
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
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_NO_BUFFERING,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	closeTray(handle)
	if !waitReadReady(driveLetter) {
		return "", fmt.Errorf("not read disc")
	}

	data, err := cdda.ReadAllSector(handle)
	if err != nil {
		return "", err
	}
	for range verifyCount {
		ejectTray(handle)
		closeTray(handle)
		if !waitReadReady(driveLetter) {
			return "", fmt.Errorf("not read disc")
		}
		result, err := cdda.ReadAllSector(handle)
		if err != nil {
			return "", nil
		}
		if !bytes.Equal(data, result) {
			return "", fmt.Errorf("verify error: not match")
		}
	}
	if offsetSample < 0 {
		offset := -offsetSample * sampleSize
		data = append(bytes.Repeat([]byte{0x00}, offset), data[:len(data)-offset]...)
	}
	if 0 < offsetSample {
		offset := offsetSample * sampleSize
		data = append(data[offset:], bytes.Repeat([]byte{0x00}, offset)...)
	}

	outFile, err := os.Create("file.wav")
	if err != nil {
		return "", err
	}
	defer outFile.Close()

	header := []byte("RIFF")
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(len(data)+36))...)
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
	header = append(header, binary.LittleEndian.AppendUint32([]byte{}, uint32(len(data)))...)
	_, err = outFile.Write(header)
	if err != nil {
		return "", err
	}
	_, err = outFile.Write(data)
	if err != nil {
		return "", err
	}

	return "finish", nil
}

func closeTray(handle windows.Handle) error {
	return windows.DeviceIoControl(
		handle,
		IOCTL_STORAGE_LOAD_MEDIA,
		nil,
		0,
		nil,
		0,
		new(uint32),
		nil,
	)
}

func ejectTray(handle windows.Handle) error {
	return windows.DeviceIoControl(
		handle,
		IOCTL_STORAGE_EJECT_MEDIA,
		nil,
		0,
		nil,
		0,
		new(uint32),
		nil,
	)
}

func waitReadReady(driveLetter string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if _, err := os.Stat(driveLetter); err == nil {
				return true
			}
		case <-ctx.Done():
			if _, err := os.Stat(driveLetter); err == nil {
				return true
			}
			return false
		}
	}
}
