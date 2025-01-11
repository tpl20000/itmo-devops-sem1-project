package main

import (
	"archive/zip"
	"encoding/csv"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

// Read a CSV file and return a PriceData struct
func ReadCSV(filePath string) (PriceData, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return PriceData{}, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.Comma = ','
	reader.Comment = '#'
	reader.FieldsPerRecord = -1

	var data PriceData
	categories := make(map[string]bool)

	// Skip the header row
	_, err = reader.Read()
	if err != nil {
		return PriceData{}, err
	}

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return PriceData{}, err
		}

		// Parse the record
		price, err := strconv.Atoi(record[3]) // Assuming price is in the 4th column
		if err != nil {
			return PriceData{}, err
		}

		data.TotalItems++
		categories[record[2]] = true // Assuming category is in the 3rd column
		data.TotalPrice += price
	}

	data.TotalCategories = len(categories)
	return data, nil
}
