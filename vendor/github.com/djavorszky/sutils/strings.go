package sutils

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// IContains returns true if the haystack contains the needle.
// It searches in a case-insensitive way
func IContains(haystack, needle string) bool {
	// short out
	if haystack == "" || needle == "" {
		return false
	}

	re := regexp.MustCompile("(?i)" + needle)
	m := re.FindString(haystack)

	if m == "" {
		return false
	}

	return true
}

// Present checks whether all of its parameters are non-empty.
func Present(reqFields ...string) bool {
	for _, field := range reqFields {
		if field == "" {
			return false
		}
	}

	return true
}

// CountIgnoreCase searches an io.Reader for a given string in a case-insensitive way.
// It returns the line numbers where it found such strings, or an error if something went wrong.
func CountIgnoreCase(haystack io.Reader, needle string) (int, error) {
	occurrences, err := FindIgnoreCase(haystack, needle)
	if err != nil {
		return 0, err
	}

	return len(occurrences), nil
}

// CountCaseSensitive searches an io.Reader for a given string in a case sensitive way.
// It returns the line numbers where it found such strings, or an error if something went wrong.
func CountCaseSensitive(haystack io.Reader, needle string) (int, error) {
	occurrences, err := FindCaseSensitive(haystack, needle)
	if err != nil {
		return 0, err
	}

	return len(occurrences), nil
}

// FindIgnoreCase searches an io.Reader for a given string in a case-insensitive way.
// It returns the line numbers where it found such strings, or an error if something went wrong.
func FindIgnoreCase(haystack io.Reader, needle string) (occurrences []int, err error) {
	lines := 1
	r := bufio.NewReader(haystack)

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("reading line: %v", err)
		}

		if IContains(string(line), needle) {
			occurrences = append(occurrences, lines)
		}

		lines++
	}

	return occurrences, nil
}

// FindCaseSensitive searches an io.Reader for a given string in a case sensitive way.
// It returns the line numbers where it found such strings, or an error if something went wrong.
func FindCaseSensitive(haystack io.Reader, needle string) (occurrences []int, err error) {
	lines := 1
	r := bufio.NewReader(haystack)

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("reading line: %v", err)
		}

		if strings.Contains(string(line), needle) {
			occurrences = append(occurrences, lines)
		}

		lines++
	}

	return occurrences, nil
}

// FindStartsWith searches an io.Reader for all lines that start with a given string in a case sensitive way.
// It returns the line numbers where it found such strings, or an error if something went wrong.
func FindStartsWith(haystack io.Reader, needle string) (occurrences []int, err error) {
	lines := 1
	r := bufio.NewReader(haystack)

	for {
		line, _, err := r.ReadLine()
		if err != nil {
			if err == io.EOF {
				break
			}

		}

		if strings.HasPrefix(string(line), needle) {
			occurrences = append(occurrences, lines)
		}

		lines++
	}

	return occurrences, nil
}

// TrimNL trims the newline from the end of the string.
func TrimNL(s string) string {
	s = strings.TrimSuffix(s, "\n")
	s = strings.TrimSuffix(s, "\r")

	return s
}

// FindWith locates and returns all occurrences of needle in the haystack.
// It does its job via the provided find function which should return true
// if the second argument is found in the first one, false otherwise.
//
// FindWith's return is indexed from 1 instead of 0.
func FindWith(find func(string, string) bool, haystack io.Reader, needles []string) ([]int, error) {
	occurrences := make([]int, 0)

	if needles[0] == "" {
		return occurrences, nil
	}

	scanner := bufio.NewScanner(haystack)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lnum := 0
	for scanner.Scan() {
		lnum++

		line := scanner.Text()

		for _, needle := range needles {
			if find(line, needle) {
				occurrences = append(occurrences, lnum)
				break
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading file: %v", err)
	}

	return occurrences, nil
}

// CopyLines copies the lines specified in the "lines" from the
// io.Reader "from" to the io.Writer "to".
//
// Replaces carriage returns with normal returns
func CopyLines(from io.Reader, lines []int, to io.Writer) error {
	if len(lines) == 0 {
		return nil
	}

	lineMap := make(map[int]bool)

	for _, l := range lines {
		lineMap[l] = true
	}

	scanner := bufio.NewScanner(from)
	lnum := 0

	for scanner.Scan() {
		line := scanner.Text()

		lnum++

		if _, ok := lineMap[lnum]; !ok {
			continue
		}

		to.Write([]byte(line))
		to.Write([]byte(fmt.Sprintln()))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading from file: %v", err)
	}

	return nil
}

// CopyWithoutLines copies from the reader "from" to the writer
// "to" without the line numbers specified by "lines".
func CopyWithoutLines(from io.Reader, lines []int, to io.Writer) error {
	lineMap := make(map[int]bool)

	for _, l := range lines {
		lineMap[l] = true
	}

	scanner := bufio.NewScanner(from)
	buf := make([]byte, 0, 10*1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	lnum := 0
	for scanner.Scan() {
		lnum++

		line := scanner.Text()

		if _, ok := lineMap[lnum]; ok {
			continue
		}

		to.Write([]byte(line))
		to.Write([]byte("\n"))
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("reading from file: %v", err)
	}

	return nil
}
