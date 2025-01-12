package main

import (
	"archive/zip"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"bytes"
)

// Unzip a ZIP file
func Unzip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Zip compresses a file and returns the resulting ZIP file as an array of bytes
func Zip(filePath string) ([]byte, error) {
	// Open the file to be zipped
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Create a new ZIP file
	zipFile := new(bytes.Buffer)

	// Create a ZIP writer
	zipWriter := zip.NewWriter(zipFile)

	// Create a new file in the ZIP archive
	f, err := zipWriter.Create(filepath.Base(filePath))
	if err != nil {
		return nil, err
	}

	// Copy the file to the ZIP archive
	_, err = io.Copy(f, file)
	if err != nil {
		return nil, err
	}

	// Close the ZIP writer
	err = zipWriter.Close()
	if err != nil {
		return nil, err
	}

	// Return the ZIP file as an array of bytes
	return zipFile.Bytes(), nil
}

// findFirstCSVFile finds the first CSV file in a directory
func findFirstCSVFile(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) == ".csv" {
			return filepath.Join(dir, file.Name()), nil
		}
	}

	return "", fmt.Errorf("no CSV file found")
}

// ReadCSV reads a CSV file and returns a slice of PriceData
func ReadCSV(filePath string) ([]PriceData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	reader.Comment = '#'
	reader.FieldsPerRecord = -1

	var data []PriceData

	// Skip the header row
	_, err = reader.Read()
	if err != nil {
		return nil, err
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Parse the record
		price, err := strconv.ParseFloat(record[3], 64)
		if err != nil {
			return nil, err
		}

		createDate, err := time.Parse("2006-01-02", record[4])
		if err != nil {
			return nil, err
		}

		// Create a PriceData instance
		priceData := PriceData{
			Name:        record[0],
			Category:    record[1],
			Price:       price,
			Create_date: createDate,
		}

		// Append the PriceData instance to the slice
		data = append(data, priceData)
	}

	return data, nil
}
