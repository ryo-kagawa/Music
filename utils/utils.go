package utils

import (
	"bytes"
	"errors"
	"io"
	"iter"
	"os"
	"strings"
	"unicode/utf8"

	"github.com/ryo-kagawa/go-utils/conditional"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

var encoders = []encoding.Encoding{
	japanese.ShiftJIS,
	japanese.EUCJP,
}

func ReadTextFileToUTF8(filePath string) (string, error) {
	binary, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	if utf8.Valid(binary) {
		return string(binary), nil
	}
	for _, encoder := range encoders {
		reader := transform.NewReader(
			bytes.NewReader(binary),
			encoder.NewDecoder(),
		)
		newBinary, err := io.ReadAll(reader)
		if err != nil {
			return "", err
		}
		if utf8.Valid(newBinary) {
			return string(newBinary), nil
		}
	}
	return "", errors.New("元の文字コードが特定できませんでした")
}

func SplitNewLineWithoutEmpty(value string) iter.Seq[string] {
	return func(yield func(string) bool) {
		for line := range strings.SplitSeq(
			strings.ReplaceAll(value, "\r\n", "\n"),
			"\n",
		) {
			if line != "" {
				if !yield(line) {
					return
				}
			}
		}
	}
}

func TrimQuotesIfWrapped(value string) string {
	return conditional.Value(
		value[0] == '"' && value[len(value)-1] == '"',
		value[1:len(value)-1],
		value,
	)
}
