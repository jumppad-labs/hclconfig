package xcl

import (
	"bufio"
	"crypto/md5"
	"fmt"
	"os"
)

// ReadFileLocation reads a file between the given locations
func ReadFileLocation(filename string, startLine, startCol, endLine, endCol int) (string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", err)
	}

	fs := bufio.NewScanner(f)

	cl := 0
	output := ""

	for fs.Scan() {
		cl++
		if cl >= startLine && cl <= endLine {
			switch cl {
			case startLine:
				if startLine == endLine {
					output += fs.Text()[startCol-1 : endCol-1]
				} else {
					output += fmt.Sprintf("%s%s", fs.Text()[startCol-1:], LineEnding)
				}
			case endLine:
				output += fs.Text()[:endCol-1]
			default:
				output += fmt.Sprintf("%s%s", fs.Text(), LineEnding)
			}
		}
	}

	return output, nil
}

// HashString creates an MD5 hash of the given string
func HashString(in string) string {
	h := md5.New()
	h.Write([]byte(in))

	return fmt.Sprintf("%x", h.Sum(nil))
}
