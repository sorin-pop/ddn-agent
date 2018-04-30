package main

import (
	"archive/tar"
	"archive/zip"
	"compress/bzip2"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

func unzip(path string) ([]string, error) {
	defer os.Remove(path)

	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("creating zip reader failed: %s", err.Error())
	}
	defer r.Close()

	var files []string
	for _, f := range r.File {
		name, err := unzipFile(f)
		if err != nil {
			return nil, fmt.Errorf("extracting zip file failed: %s", err.Error())
		}

		files = append(files, name)
	}

	return files, nil
}

func unzipFile(f *zip.File) (string, error) {
	src, err := f.Open()
	if err != nil {
		return "", fmt.Errorf("opening zipfile failed: %s", err.Error())
	}
	defer src.Close()

	dst, err := os.OpenFile(f.Name, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return "", fmt.Errorf("opening destination file failed: %s", err.Error())
	}
	defer dst.Close()

	_, err = io.Copy(dst, src)
	if err != nil {
		return "", fmt.Errorf("copying from archive failed: %s", err.Error())
	}

	return f.Name, nil
}

func ungzip(path string) ([]string, error) {
	defer os.Remove(path)

	reader, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening gzipfile failed: %s", err.Error())
	}
	defer reader.Close()

	archive, err := gzip.NewReader(reader)
	if err != nil {
		return nil, fmt.Errorf("creating gzip reader failed: %s", err.Error())
	}
	defer archive.Close()

	ext := filepath.Ext(path)
	name := archive.Header.Name
	if name == "" {
		dstName := filepath.Base(path)

		name = dstName[:len(dstName)-len(ext)]
	}

	dst, err := os.Create(name)
	if err != nil {
		return nil, fmt.Errorf("could not create output file: %s", err.Error())
	}
	defer dst.Close()

	_, err = io.Copy(dst, archive)
	if err != nil {
		return nil, fmt.Errorf("uncompressing gzip failed: %s", err.Error())
	}

	if filepath.Ext(dst.Name()) == ".tar" {
		dst.Close()
		return untar(fmt.Sprintf("%s/%s", filepath.Dir(dst.Name()), dst.Name()))
	}

	return []string{dst.Name()}, nil
}

func unbzip2(path string) ([]string, error) {
	defer os.Remove(path)

	reader, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening bzip2 file failed: %s", err.Error())
	}
	defer reader.Close()

	archive := bzip2.NewReader(reader)

	ext := filepath.Ext(path)
	dstName := filepath.Base(path)

	name := dstName[:len(dstName)-len(ext)]

	dst, err := os.Create(name)
	if err != nil {
		return nil, fmt.Errorf("could not create output file: %s", err.Error())
	}
	defer dst.Close()

	_, err = io.Copy(dst, archive)
	if err != nil {
		return nil, fmt.Errorf("uncompressing bzip2 failed: %s", err.Error())
	}

	if filepath.Ext(dst.Name()) == ".tar" {
		dst.Close()
		return untar(fmt.Sprintf("%s/%s", filepath.Dir(dst.Name()), dst.Name()))
	}

	return []string{dst.Name()}, nil
}

func untar(path string) ([]string, error) {
	defer os.Remove(path)

	file, err := os.Open(path)

	if err != nil {
		return nil, fmt.Errorf("opening tarball failed: %s", err.Error())
	}
	defer file.Close()

	var files []string

	tarBallReader := tar.NewReader(file)

	for {
		header, err := tarBallReader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}

			return nil, fmt.Errorf("encountered error while reading tarball: %s", err.Error())
		}

		filename := header.Name

		switch header.Typeflag {
		case tar.TypeDir:
			if err != nil {
				return nil, fmt.Errorf("tarball contains folder, stopping")
			}
		case tar.TypeReg:
			writer, err := os.Create(filename)
			if err != nil {
				return nil, fmt.Errorf("could not create output file: %s", err.Error())
			}
			defer writer.Close()

			_, err = io.Copy(writer, tarBallReader)
			if err != nil {
				return nil, fmt.Errorf("uncompressing tarball failed: %s", err.Error())
			}

			files = append(files, writer.Name())
		default:
			fmt.Printf("Unable to untar type : %c in file %s", header.Typeflag, filename)
		}
	}

	return files, nil
}

func isArchive(path string) bool {
	switch filepath.Ext(path) {
	case ".zip", ".tar", ".gz", ".bz2":
		return true
	}

	return false
}

func zipFiles(outputZipFilename string, inputFiles []string) error {

	newfile, err := os.Create(outputZipFilename)
	if err != nil {
		return err
	}
	defer newfile.Close()

	zipWriter := zip.NewWriter(newfile)
	defer zipWriter.Close()

	// Add inputFiles to zip
	for _, file := range inputFiles {

		zipfile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer zipfile.Close()

		// Get the file information
		info, err := zipfile.Stat()
		if err != nil {
			return err
		}

		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}

		// Change to deflate to gain better compression
		// see http://golang.org/pkg/archive/zip/#pkg-constants
		header.Method = zip.Deflate

		writer, err := zipWriter.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(writer, zipfile)
		if err != nil {
			return err
		}
	}
	return nil
}
