package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ryo-kagawa/Music/types/cue"
	"github.com/ryo-kagawa/go-utils/commandline"
)

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	cuePath := arguments[0]
	cueFile, err := cue.Load(cuePath)
	if err != nil {
		return "", err
	}
	cueFile = cueFile.SplitTrack()
	outputDirectory := filepath.Join(filepath.Dir(cuePath), cue.TitleToFileName(cueFile.Album.Field.Title))
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return "", err
	}
	if err := cueFile.OutputWave(outputDirectory); err != nil {
		return "", err
	}
	outputPath := filepath.Join(outputDirectory, fmt.Sprintf("%s.cue", cue.TitleToFileName(cueFile.Album.Field.Title)))
	if err := cueFile.OutputCuefile(outputPath); err != nil {
		return "", err
	}

	return outputPath, nil
}
