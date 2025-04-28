package cue

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ryo-kagawa/Music/utils"
	"github.com/ryo-kagawa/go-utils/conditional"
)

// CDDAからWAVE変換を行った前提で処理する

// 44.1kHz
const samplingRate = 44100

// 16bit = 2Byte
const samplingSize = 16 / 8

// 2CH
const channels = 2

// 75フレーム
const frames = 75

// 44.1kHz * 16bit量子化 * ステレオ / フレーム数
const FrameSize = samplingRate * samplingSize * channels / frames
const HeaderSize = 44

type Track struct {
	Number    int
	Title     string
	Performer string
	Rems      struct {
		Composer     string
		Lyricist     string
		Arranger     string
		Remixer      string
		BackingVocal string
	}
	Index00 string
	Index01 string
}

type File struct {
	Name   string
	Type   string
	Binary []byte
	Tracks []Track
}

// CUEというよりアルバムフィールド
type Cue struct {
	Rems struct {
		Genre      string
		Date       string
		DiscId     string
		DiscNumber string
		Composer   string
		Comment    string
	}
	Catalog   string
	Title     string
	Performer string
	Files     []File
}

func Load(cueFilepath string) (Cue, error) {
	cueData, err := utils.ReadTextFileToUTF8(cueFilepath)
	if err != nil {
		return Cue{}, err
	}
	albumFieldFlag := true
	cue := Cue{}
	currentFile := &File{}
	currentTrack := &Track{}

	for line := range utils.SplitNewLine(cueData) {
		line = strings.TrimLeft(line, " ")
		// NOTE: FILEはトラックフィールド行以降にも存在するアルバムフィールドなので例外的に処理する
		if strings.HasPrefix(line, "FILE ") {
			// NOTE: ファイル名に空白がある場合に対応するため、" "で分割できない
			FileField := strings.TrimPrefix(line, "FILE ")
			switch {
			case strings.HasSuffix(FileField, " WAVE"):
				fileName := utils.TrimQuotesIfWrapped(strings.TrimSuffix(FileField, " WAVE"))
				waveFile, err := os.ReadFile(filepath.Join(filepath.Dir(cueFilepath), fileName))
				if err != nil {
					return Cue{}, err
				}
				if len(waveFile) < 44 {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// RIFF識別子の確認
				if !bytes.Equal(waveFile[:4], []byte("RIFF")) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// WAVE識別子の確認
				if !bytes.Equal(waveFile[8:12], []byte("WAVE")) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// fmt識別子の確認
				if !bytes.Equal(waveFile[12:16], []byte("fmt ")) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// fmtチャンクサイズ
				if !bytes.Equal(waveFile[16:20], []byte{0x10, 0x00, 0x00, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// フォーマットタイプの確認
				if !bytes.Equal(waveFile[20:22], []byte{0x01, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// チャンネル数の確認
				if !bytes.Equal(waveFile[22:24], []byte{0x02, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// サンプリングレートの確認
				if !bytes.Equal(waveFile[24:28], []byte{0x44, 0xAC, 0x00, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// バイトレート
				if !bytes.Equal(waveFile[28:32], []byte{0x10, 0xB1, 0x02, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// ブロックアラインメント
				if !bytes.Equal(waveFile[32:34], []byte{0x04, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// ビット深度
				if !bytes.Equal(waveFile[34:36], []byte{0x10, 0x00}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				// data識別子
				if !bytes.Equal(waveFile[36:40], []byte{0x64, 0x61, 0x74, 0x61}) {
					return Cue{}, errors.New("WAVEフォーマットエラー")
				}
				cue.Files = append(
					cue.Files,
					File{
						Name:   fileName,
						Type:   "WAVE",
						Binary: waveFile,
					},
				)
			default:
				return Cue{}, errors.New("CUEファイルが不正です")
			}
			currentFile = &cue.Files[len(cue.Files)-1]
			albumFieldFlag = false
			continue
		}
		if albumFieldFlag {
			// NOTE: アルバムフィールド
			switch {
			case strings.HasPrefix(line, "REM "):
				remField := strings.TrimPrefix(line, "REM ")
				switch {
				case strings.HasPrefix(remField, "GENRE "):
					cue.Rems.Genre = strings.TrimPrefix(remField, "GENRE ")
				case strings.HasPrefix(remField, "DATE "):
					cue.Rems.Date = strings.TrimPrefix(remField, "DATE ")
				case strings.HasPrefix(remField, "DISCID "):
					cue.Rems.DiscId = strings.TrimPrefix(remField, "DISCID ")
				case strings.HasPrefix(remField, "DISCNUMBER "):
					cue.Rems.DiscId = strings.TrimPrefix(remField, "DISCNUMBER ")
				case strings.HasPrefix(remField, "COMMENT "):
					cue.Rems.Comment = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "COMMENT "))
				case strings.HasPrefix(remField, "COMPOSER "):
					cue.Rems.Composer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "COMPOSER "))
				default:
					return Cue{}, errors.New(fmt.Sprintf("%sに未対応です", line))
				}
			case strings.HasPrefix(line, "TITLE "):
				cue.Title = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "TITLE "))
			case strings.HasPrefix(line, "PERFORMER "):
				cue.Performer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "PERFORMER "))
			}
			continue
		}
		// NOTE: トラックフィールド
		switch {
		case strings.HasPrefix(line, "TRACK "):
			trackField := strings.TrimPrefix(line, "TRACK ")
			switch {
			case strings.HasSuffix(trackField, " AUDIO"):
				number, err := strconv.Atoi(strings.TrimSuffix(trackField, " AUDIO"))
				if err != nil {
					return Cue{}, err
				}
				currentFile.Tracks = append(
					currentFile.Tracks,
					Track{
						Number: number,
					},
				)
			default:
				return Cue{}, errors.New("CUEファイルが不正です")
			}
			currentTrack = &currentFile.Tracks[len(currentFile.Tracks)-1]
		case strings.HasPrefix(line, "TITLE "):
			currentTrack.Title = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "TITLE "))
		case strings.HasPrefix(line, "PERFORMER "):
			currentTrack.Performer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "PERFORMER "))
		case strings.HasPrefix(line, "REM "):
			remField := strings.TrimPrefix(line, "REM ")
			switch {
			case strings.HasPrefix(remField, "COMPOSER "):
				currentTrack.Rems.Composer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "COMPOSER "))
			case strings.HasPrefix(remField, "LYRICIST "):
				currentTrack.Rems.Lyricist = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "LYRICIST "))
			case strings.HasPrefix(remField, "ARRANGER "):
				currentTrack.Rems.Arranger = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "ARRANGER "))
			case strings.HasPrefix(remField, "REMIXER "):
				currentTrack.Rems.Remixer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "REMIXER "))
			case strings.HasPrefix(remField, "BACKING_VOCAL "):
				currentTrack.Rems.BackingVocal = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "BACKING_VOCAL "))
			default:
				return Cue{}, errors.New(fmt.Sprintf("%sに未対応です", line))
			}
		case strings.HasPrefix(line, "INDEX 00"):
			currentTrack.Index00 = strings.TrimPrefix(line, "INDEX 00 ")
		case strings.HasPrefix(line, "INDEX 01"):
			currentTrack.Index01 = strings.TrimPrefix(line, "INDEX 01 ")
		}
	}
	return cue, nil
}

func (c Cue) SplitTrack() Cue {
	cue := Cue{}
	cue.Rems.Genre = c.Rems.Genre
	cue.Rems.Date = c.Rems.Date
	cue.Rems.DiscId = c.Rems.DiscId
	cue.Rems.DiscNumber = c.Rems.DiscNumber
	cue.Rems.Composer = c.Rems.Composer
	cue.Rems.Comment = c.Rems.Comment
	cue.Catalog = c.Catalog
	cue.Title = c.Title
	cue.Performer = c.Performer
	for _, file := range c.Files {
		for trackIndex, track := range file.Tracks {
			mm, _ := strconv.Atoi(track.Index01[0:2])
			ss, _ := strconv.Atoi(track.Index01[3:5])
			ff, _ := strconv.Atoi(track.Index01[6:8])
			start := HeaderSize + ((mm*60+ss)*75+ff)*FrameSize
			end := conditional.Func(
				trackIndex != len(file.Tracks)-1,
				func() int {
					index := conditional.Value(file.Tracks[trackIndex+1].Index00 != "", file.Tracks[trackIndex+1].Index00, file.Tracks[trackIndex+1].Index01)
					mm, _ := strconv.Atoi(index[0:2])
					ss, _ := strconv.Atoi(index[3:5])
					ff, _ := strconv.Atoi(index[6:8])
					return HeaderSize + ((mm*60+ss)*75+ff)*FrameSize
				},
				func() int {
					return len(file.Binary)
				},
			)
			header := make([]byte, HeaderSize)
			copy(header[0:4], file.Binary[:4])
			binary.LittleEndian.PutUint32(header[4:8], uint32(end-start+HeaderSize-8))
			copy(header[8:40], file.Binary[8:40])
			binary.LittleEndian.PutUint32(header[40:], uint32(end-start))
			newTrack := track
			newTrack.Index00 = ""
			newTrack.Index01 = "00:00:00"
			cue.Files = append(
				cue.Files,
				File{
					Name:   fmt.Sprintf("%02d %s.wav", track.Number, track.Title),
					Type:   "WAVE",
					Binary: append(header, file.Binary[start:end]...),
					Tracks: []Track{
						newTrack,
					},
				},
			)
		}
	}
	return cue
}

func (c Cue) OutputWave(outputDirectory string) error {
	for _, file := range c.Files {
		if filepath.Ext(file.Name) == ".wav" {
			outPath := filepath.Join(outputDirectory, file.Name)
			err := os.WriteFile(outPath, file.Binary, 0644)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (c Cue) OutputCuefile(outputPath string) error {
	output := ""
	if c.Rems.Genre != "" {
		output += fmt.Sprintf("REM GENRE %s\n", c.Rems.Genre)
	}
	if c.Rems.Date != "" {
		output += fmt.Sprintf("REM DATE %s\n", c.Rems.Date)
	}
	if c.Rems.DiscId != "" {
		output += fmt.Sprintf("REM DISCID %s\n", c.Rems.DiscId)
	}
	if c.Rems.DiscNumber != "" {
		output += fmt.Sprintf("REM DISCNUMBER %s\n", c.Rems.DiscNumber)
	}
	if c.Rems.Comment != "" {
		output += fmt.Sprintf("REM COMMENT \"%s\"\n", c.Rems.Comment)
	}
	if c.Catalog != "" {
		output += fmt.Sprintf("CATALOG %s\n", c.Catalog)
	}
	if c.Title != "" {
		output += fmt.Sprintf("TITLE \"%s\"\n", c.Title)
	}
	if c.Performer != "" {
		output += fmt.Sprintf("PERFORMER \"%s\"\n", c.Performer)
	}
	for _, file := range c.Files {
		output += fmt.Sprintf("FILE \"%s\" WAVE\n", file.Name)
		for _, track := range file.Tracks {
			output += fmt.Sprintf("  TRACK %02d AUDIO\n", track.Number)
			output += fmt.Sprintf("    TITLE \"%s\"\n", track.Title)
			if track.Performer != "" {
				output += fmt.Sprintf("    PERFORMER \"%s\"\n", track.Performer)
			}
			if track.Rems.Composer != "" {
				output += fmt.Sprintf("    REM COMPOSER \"%s\"\n", track.Rems.Composer)
			}
			if track.Rems.Lyricist != "" {
				output += fmt.Sprintf("    REM LYRICIST \"%s\"\n", track.Rems.Lyricist)
			}
			if track.Rems.Arranger != "" {
				output += fmt.Sprintf("    REM ARRANGER \"%s\"\n", track.Rems.Arranger)
			}
			if track.Rems.Remixer != "" {
				output += fmt.Sprintf("    REM REMIXER \"%s\"\n", track.Rems.Remixer)
			}
			if track.Rems.BackingVocal != "" {
				output += fmt.Sprintf("    REM BACKING_VOCAL \"%s\"\n", track.Rems.BackingVocal)
			}
			if track.Index00 != "" {
				output += fmt.Sprintf("    INDEX 00 %s\n", track.Index00)
			}
			if track.Index01 != "" {
				output += fmt.Sprintf("    INDEX 01 %s\n", track.Index01)
			}
		}
	}

	err := os.WriteFile(outputPath, []byte(output), 0644)
	if err != nil {
		return err
	}
	return nil
}
