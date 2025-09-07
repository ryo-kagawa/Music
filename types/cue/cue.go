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
const bitDepth = 16 / 8

// 2CH
const channels = 2

// 75フレーム
const frames = 75

// 44.1kHz * 16bit量子化 * ステレオ / フレーム数
const FrameSize = samplingRate * bitDepth * channels / frames
const HeaderSize = 44

type TrackSubCommand struct {
	Isrc  string
	Index struct {
		Index00 string
		Index01 string
	}
}

type TrackCommand struct {
	Track      int
	SubCommand TrackSubCommand
}

type TrackField struct {
	Title     string
	Performer string
	Rem       struct {
		// 作曲者
		Composer string
		// 作詞者
		Lyricist          string
		Guitar            string
		ElectricGuitar    string
		Bass              string
		ElectricBass      string
		Keyboards         string
		Synthesizer       string
		AnalogSynthesizer string
		Horn              string
		Drums             string
		Percussions       string
		// 編曲者
		Arranger string
		// リミックス者
		Remixer      string
		Vocal        string
		BackingVocal string
	}
	Flags struct {
		// DCP
		DigitalCopyPermitted bool
		// 4CH
		FourChannelAudio bool
		// PRE
		PreEmphasisEnabled bool
		// SCMS
		SerialCopyManagementSystem bool
	}
}

type Track struct {
	Command TrackCommand
	Field   TrackField
}

type File struct {
	Name   string
	Type   string
	Binary []byte
	Tracks []Track
}

type AlbumCommand struct {
	Files []File
}

type AlbumField struct {
	Rem struct {
		Genre string
		Date  string
		// 販売元
		Publisher string
		Label     string
		Producer  string
		// 著作
		Production string
		// 製作
		Work string
		// BGM製作
		BGMWork string
		// BGM監督
		BGMDirector string
		Composer    string
		DiscNumber  string
		TotalDiscs  string
		DiscId      string
		Jan         string
		Comment     string
	}
	// 型番
	Catalog   string
	Title     string
	Performer string
}

type Album struct {
	Command AlbumCommand
	Field   AlbumField
}

type Cue struct {
	Album Album
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

	for line := range utils.SplitNewLineWithoutEmpty(cueData) {
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
				cue.Album.Command.Files = append(
					cue.Album.Command.Files,
					File{
						Name:   fileName,
						Type:   "WAVE",
						Binary: waveFile,
					},
				)
			default:
				return Cue{}, errors.New("CUEファイルが不正です")
			}
			currentFile = &cue.Album.Command.Files[len(cue.Album.Command.Files)-1]
			albumFieldFlag = false
			continue
		}
		if albumFieldFlag {
			// NOTE: アルバム
			switch {
			case strings.HasPrefix(line, "REM "):
				remField := strings.TrimPrefix(line, "REM ")
				switch {
				case strings.HasPrefix(remField, "GENRE "):
					cue.Album.Field.Rem.Genre = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "GENRE "))
				case strings.HasPrefix(remField, "DATE "):
					cue.Album.Field.Rem.Date = strings.TrimPrefix(remField, "DATE ")
				case strings.HasPrefix(remField, "PUBLISHER "):
					cue.Album.Field.Rem.Publisher = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "PUBLISHER "))
				case strings.HasPrefix(remField, "LABEL "):
					cue.Album.Field.Rem.Label = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "LABEL "))
				case strings.HasPrefix(remField, "PRODUCER "):
					cue.Album.Field.Rem.Producer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "PRODUCER "))
				case strings.HasPrefix(remField, "PRODUCTION "):
					cue.Album.Field.Rem.Production = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "PRODUCTION "))
				case strings.HasPrefix(remField, "WORK "):
					cue.Album.Field.Rem.Work = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "WORK "))
				case strings.HasPrefix(remField, "BGM_WORK "):
					cue.Album.Field.Rem.BGMWork = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "BGM_WORK "))
				case strings.HasPrefix(remField, "BGM_DIRECTOR "):
					cue.Album.Field.Rem.BGMDirector = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "BGM_DIRECTOR "))
				case strings.HasPrefix(remField, "COMPOSER "):
					cue.Album.Field.Rem.Composer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "COMPOSER "))
				case strings.HasPrefix(remField, "DISCNUMBER "):
					cue.Album.Field.Rem.DiscId = strings.TrimPrefix(remField, "DISCNUMBER ")
				case strings.HasPrefix(remField, "TOTALDISCS "):
					cue.Album.Field.Rem.DiscId = strings.TrimPrefix(remField, "TOTALDISCS ")
				case strings.HasPrefix(remField, "DISCID "):
					cue.Album.Field.Rem.DiscId = strings.TrimPrefix(remField, "DISCID ")
				case strings.HasPrefix(remField, "JAN "):
					cue.Album.Field.Rem.Jan = strings.TrimPrefix(remField, "JAN ")
				case strings.HasPrefix(remField, "COMMENT "):
					cue.Album.Field.Rem.Comment = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "COMMENT "))
				default:
					return Cue{}, fmt.Errorf("アルバムフィールドの\"%s\"に未対応です", line)
				}
			case strings.HasPrefix(line, "CATALOG "):
				cue.Album.Field.Catalog = strings.TrimPrefix(line, "CATALOG ")
			case strings.HasPrefix(line, "TITLE "):
				// NOTE: 「""」となっている場合は「"」に変換して扱う
				cue.Album.Field.Title = strings.ReplaceAll(utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "TITLE ")), "\"\"", "\"")
			case strings.HasPrefix(line, "PERFORMER "):
				cue.Album.Field.Performer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "PERFORMER "))
			default:
				return Cue{}, fmt.Errorf("アルバムフィールドの\"%s\"に未対応です", line)
			}
			continue
		}
		// NOTE: トラック
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
						Command: TrackCommand{
							Track: number,
						},
					},
				)
			default:
				return Cue{}, errors.New("CUEファイルが不正です")
			}
			currentTrack = &currentFile.Tracks[len(currentFile.Tracks)-1]
		case strings.HasPrefix(line, "ISRC "):
			currentTrack.Command.SubCommand.Isrc = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "ISRC "))
		case strings.HasPrefix(line, "TITLE "):
			// NOTE: 「""」となっている場合は「"」に変換して扱う
			currentTrack.Field.Title = strings.ReplaceAll(utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "TITLE ")), "\"\"", "\"")
		case strings.HasPrefix(line, "PERFORMER "):
			currentTrack.Field.Performer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "PERFORMER "))
		case strings.HasPrefix(line, "REM "):
			remField := strings.TrimPrefix(line, "REM ")
			switch {
			case strings.HasPrefix(remField, "COMPOSER "):
				currentTrack.Field.Rem.Composer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "COMPOSER "))
			case strings.HasPrefix(remField, "LYRICIST "):
				currentTrack.Field.Rem.Lyricist = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "LYRICIST "))
			case strings.HasPrefix(remField, "GUITAR "):
				currentTrack.Field.Rem.Guitar = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "GUITAR "))
			case strings.HasPrefix(remField, "ELECTRIC_GUITAR "):
				currentTrack.Field.Rem.ElectricGuitar = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "ELECTRIC_GUITAR "))
			case strings.HasPrefix(remField, "BASS "):
				currentTrack.Field.Rem.Bass = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "BASS "))
			case strings.HasPrefix(remField, "ELECTRIC_BASS "):
				currentTrack.Field.Rem.ElectricBass = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "ELECTRIC_BASS "))
			case strings.HasPrefix(remField, "KEYBOARDS "):
				currentTrack.Field.Rem.Keyboards = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "KEYBOARDS "))
			case strings.HasPrefix(remField, "SYNTHESIZER "):
				currentTrack.Field.Rem.Synthesizer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "SYNTHESIZER "))
			case strings.HasPrefix(remField, "ANALOG_SYNTHESIZER "):
				currentTrack.Field.Rem.AnalogSynthesizer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "ANALOG_SYNTHESIZER "))
			case strings.HasPrefix(remField, "HORN "):
				currentTrack.Field.Rem.Horn = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "HORN "))
			case strings.HasPrefix(remField, "DRUMS "):
				currentTrack.Field.Rem.Drums = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "DRUMS "))
			case strings.HasPrefix(remField, "PERCUSSIONS "):
				currentTrack.Field.Rem.Percussions = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "PERCUSSIONS "))
			case strings.HasPrefix(remField, "ARRANGER "):
				currentTrack.Field.Rem.Arranger = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "ARRANGER "))
			case strings.HasPrefix(remField, "REMIXER "):
				currentTrack.Field.Rem.Remixer = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "REMIXER "))
			case strings.HasPrefix(remField, "VOCAL "):
				currentTrack.Field.Rem.Vocal = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "VOCAL "))
			case strings.HasPrefix(remField, "BACKING_VOCAL "):
				currentTrack.Field.Rem.BackingVocal = utils.TrimQuotesIfWrapped(strings.TrimPrefix(remField, "BACKING_VOCAL "))
			default:
				return Cue{}, fmt.Errorf("トラックフィールドの\"%s\"に未対応です", line)
			}
		case strings.HasPrefix(line, "FLAGS "):
			flags := utils.TrimQuotesIfWrapped(strings.TrimPrefix(line, "FLAGS "))
			for flag := range strings.SplitSeq(flags, " ") {
				switch flag {
				case "DCP":
					currentTrack.Field.Flags.DigitalCopyPermitted = true
				case "4CH":
					currentTrack.Field.Flags.FourChannelAudio = true
				case "PRE":
					currentTrack.Field.Flags.PreEmphasisEnabled = true
				case "SCMS":
					currentTrack.Field.Flags.SerialCopyManagementSystem = true
				}
			}
		case strings.HasPrefix(line, "INDEX "):
			indexParameter := strings.TrimPrefix(line, "INDEX ")
			switch {
			case strings.HasPrefix(indexParameter, "00 "):
				currentTrack.Command.SubCommand.Index.Index00 = strings.TrimPrefix(line, "INDEX 00 ")
			case strings.HasPrefix(indexParameter, "01 "):
				currentTrack.Command.SubCommand.Index.Index01 = strings.TrimPrefix(line, "INDEX 01 ")
			default:
				return Cue{}, fmt.Errorf("トラックフィールドの\"%s\"に未対応です", line)
			}
		default:
			return Cue{}, fmt.Errorf("トラックフィールドの\"%s\"に未対応です", line)
		}
	}
	return cue, nil
}

func (c Cue) SplitTrack() Cue {
	cue := c
	cue.Album.Command.Files = []File{}
	for _, file := range c.Album.Command.Files {
		for trackIndex, track := range file.Tracks {
			mm, _ := strconv.Atoi(track.Command.SubCommand.Index.Index01[0:2])
			ss, _ := strconv.Atoi(track.Command.SubCommand.Index.Index01[3:5])
			ff, _ := strconv.Atoi(track.Command.SubCommand.Index.Index01[6:8])
			start := HeaderSize + ((mm*60+ss)*75+ff)*FrameSize
			end := conditional.Func(
				trackIndex != len(file.Tracks)-1,
				func() int {
					index := conditional.Value(
						file.Tracks[trackIndex+1].Command.SubCommand.Index.Index00 != "",
						file.Tracks[trackIndex+1].Command.SubCommand.Index.Index00,
						file.Tracks[trackIndex+1].Command.SubCommand.Index.Index01,
					)
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
			newTrack.Command.SubCommand.Index.Index00 = ""
			newTrack.Command.SubCommand.Index.Index01 = "00:00:00"
			cue.Album.Command.Files = append(
				cue.Album.Command.Files,
				File{
					Name:   fmt.Sprintf("%02d %s.wav", track.Command.Track, TitleToFileName(track.Field.Title)),
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
	for _, file := range c.Album.Command.Files {
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
	if c.Album.Field.Rem.Genre != "" {
		output += fmt.Sprintf("REM GENRE \"%s\"\n", c.Album.Field.Rem.Genre)
	}
	if c.Album.Field.Rem.Date != "" {
		output += fmt.Sprintf("REM DATE %s\n", c.Album.Field.Rem.Date)
	}
	if c.Album.Field.Rem.Publisher != "" {
		output += fmt.Sprintf("REM PUBLISHER \"%s\"\n", c.Album.Field.Rem.Publisher)
	}
	if c.Album.Field.Rem.Label != "" {
		output += fmt.Sprintf("REM LABEL \"%s\"\n", c.Album.Field.Rem.Label)
	}
	if c.Album.Field.Rem.Producer != "" {
		output += fmt.Sprintf("REM PRODUCER \"%s\"\n", c.Album.Field.Rem.Producer)
	}
	if c.Album.Field.Rem.Production != "" {
		output += fmt.Sprintf("REM PRODUCTION \"%s\"\n", c.Album.Field.Rem.Production)
	}
	if c.Album.Field.Rem.Work != "" {
		output += fmt.Sprintf("REM WORK \"%s\"\n", c.Album.Field.Rem.Work)
	}
	if c.Album.Field.Rem.BGMWork != "" {
		output += fmt.Sprintf("REM BGM_WORK \"%s\"\n", c.Album.Field.Rem.BGMWork)
	}
	if c.Album.Field.Rem.BGMDirector != "" {
		output += fmt.Sprintf("REM BGM_DIRECTOR \"%s\"\n", c.Album.Field.Rem.BGMDirector)
	}
	if c.Album.Field.Rem.Composer != "" {
		output += fmt.Sprintf("REM COMPOSER \"%s\"\n", c.Album.Field.Rem.Composer)
	}
	if c.Album.Field.Rem.DiscNumber != "" {
		output += fmt.Sprintf("REM DISCNUMBER %s\n", c.Album.Field.Rem.DiscNumber)
	}
	if c.Album.Field.Rem.TotalDiscs != "" {
		output += fmt.Sprintf("REM TOTALDISCS %s\n", c.Album.Field.Rem.TotalDiscs)
	}
	if c.Album.Field.Rem.DiscId != "" {
		output += fmt.Sprintf("REM DISCID %s\n", c.Album.Field.Rem.DiscId)
	}
	if c.Album.Field.Rem.Jan != "" {
		output += fmt.Sprintf("REM JAN %s\n", c.Album.Field.Rem.Jan)
	}
	if c.Album.Field.Rem.Comment != "" {
		output += fmt.Sprintf("REM COMMENT \"%s\"\n", c.Album.Field.Rem.Comment)
	}
	if c.Album.Field.Catalog != "" {
		output += fmt.Sprintf("CATALOG %s\n", c.Album.Field.Catalog)
	}
	if c.Album.Field.Title != "" {
		output += fmt.Sprintf("TITLE \"%s\"\n", c.Album.Field.Title)
	}
	if c.Album.Field.Performer != "" {
		output += fmt.Sprintf("PERFORMER \"%s\"\n", c.Album.Field.Performer)
	}
	for _, file := range c.Album.Command.Files {
		output += fmt.Sprintf("FILE \"%s\" WAVE\n", file.Name)
		for _, track := range file.Tracks {
			output += fmt.Sprintf("  TRACK %02d AUDIO\n", track.Command.Track)
			if track.Command.SubCommand.Isrc != "" {
				output += fmt.Sprintf("    ISRC %s\n", track.Command.SubCommand.Isrc)
			}
			// NOTE: 出力時に「"」を「""」とする
			output += fmt.Sprintf("    TITLE \"%s\"\n", strings.ReplaceAll(track.Field.Title, "\"", "\"\""))
			if track.Field.Performer != "" {
				output += fmt.Sprintf("    PERFORMER \"%s\"\n", track.Field.Performer)
			}
			if track.Field.Rem.Composer != "" {
				output += fmt.Sprintf("    REM COMPOSER \"%s\"\n", track.Field.Rem.Composer)
			}
			if track.Field.Rem.Lyricist != "" {
				output += fmt.Sprintf("    REM LYRICIST \"%s\"\n", track.Field.Rem.Lyricist)
			}
			if track.Field.Rem.Guitar != "" {
				output += fmt.Sprintf("    REM GUITAR \"%s\"\n", track.Field.Rem.Guitar)
			}
			if track.Field.Rem.ElectricGuitar != "" {
				output += fmt.Sprintf("    REM ELECTRIC_GUITAR \"%s\"\n", track.Field.Rem.ElectricGuitar)
			}
			if track.Field.Rem.Bass != "" {
				output += fmt.Sprintf("    REM BASS \"%s\"\n", track.Field.Rem.Bass)
			}
			if track.Field.Rem.ElectricBass != "" {
				output += fmt.Sprintf("    REM ELECTRIC_BASS \"%s\"\n", track.Field.Rem.ElectricBass)
			}
			if track.Field.Rem.Keyboards != "" {
				output += fmt.Sprintf("    REM KEYBOARDS \"%s\"\n", track.Field.Rem.Keyboards)
			}
			if track.Field.Rem.Synthesizer != "" {
				output += fmt.Sprintf("    REM SYNTHESIZER \"%s\"\n", track.Field.Rem.Synthesizer)
			}
			if track.Field.Rem.AnalogSynthesizer != "" {
				output += fmt.Sprintf("    REM ANALOG_SYNTHESIZER \"%s\"\n", track.Field.Rem.AnalogSynthesizer)
			}
			if track.Field.Rem.Horn != "" {
				output += fmt.Sprintf("    REM HORN \"%s\"\n", track.Field.Rem.Horn)
			}
			if track.Field.Rem.Drums != "" {
				output += fmt.Sprintf("    REM DRUMS \"%s\"\n", track.Field.Rem.Drums)
			}
			if track.Field.Rem.Percussions != "" {
				output += fmt.Sprintf("    REM PERCUSSIONS \"%s\"\n", track.Field.Rem.Percussions)
			}
			if track.Field.Rem.Arranger != "" {
				output += fmt.Sprintf("    REM ARRANGER \"%s\"\n", track.Field.Rem.Arranger)
			}
			if track.Field.Rem.Remixer != "" {
				output += fmt.Sprintf("    REM REMIXER \"%s\"\n", track.Field.Rem.Remixer)
			}
			if track.Field.Rem.Vocal != "" {
				output += fmt.Sprintf("    REM VOCAL \"%s\"\n", track.Field.Rem.Vocal)
			}
			if track.Field.Rem.BackingVocal != "" {
				output += fmt.Sprintf("    REM BACKING_VOCAL \"%s\"\n", track.Field.Rem.BackingVocal)
			}
			flags := []string{}
			if track.Field.Flags.DigitalCopyPermitted {
				flags = append(flags, "DCP")
			}
			if track.Field.Flags.FourChannelAudio {
				flags = append(flags, "4CH")
			}
			if track.Field.Flags.PreEmphasisEnabled {
				flags = append(flags, "PRE")
			}
			if track.Field.Flags.SerialCopyManagementSystem {
				flags = append(flags, "SCMS")
			}
			if len(flags) != 0 {
				output += fmt.Sprintf("    FLAGS %s\n", strings.Join(flags, " "))
			}
			if track.Command.SubCommand.Index.Index00 != "" {
				output += fmt.Sprintf("    INDEX 00 %s\n", track.Command.SubCommand.Index.Index00)
			}
			if track.Command.SubCommand.Index.Index01 != "" {
				output += fmt.Sprintf("    INDEX 01 %s\n", track.Command.SubCommand.Index.Index01)
			}
		}
	}

	err := os.WriteFile(outputPath, []byte(output), 0644)
	if err != nil {
		return err
	}
	return nil
}

var titleToFileNameReplacer = strings.NewReplacer(
	"/", "_",
	"\"", "_",
)

func TitleToFileName(title string) string {
	return titleToFileNameReplacer.Replace(title)
}
