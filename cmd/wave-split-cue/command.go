package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ryo-kagawa/Music/types/cue"
	"github.com/ryo-kagawa/go-utils/commandline"
)

type Command struct{}

var _ = (commandline.RootCommand)(Command{})

func (Command) Execute(arguments []string) (string, error) {
	cuePath := arguments[0]
	cue, err := cue.Load(cuePath)
	if err != nil {
		return "", err
	}
	cue = cue.SplitTrack()
	outputDirectory := filepath.Join(filepath.Dir(cuePath), strings.ReplaceAll(cue.Album.Field.Title, "/", "_"))
	if err := os.MkdirAll(outputDirectory, 0755); err != nil {
		return "", err
	}
	if err := cue.OutputWave(outputDirectory); err != nil {
		return "", err
	}
	outputPath := filepath.Join(outputDirectory, fmt.Sprintf("%s.cue", strings.ReplaceAll(cue.Album.Field.Title, "/", "_")))
	if err := cue.OutputCuefile(outputPath); err != nil {
		return "", err
	}

	return outputPath, nil
}
