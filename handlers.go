package main

import (
	"archive/zip"
	"encoding/csv"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
)

type PriceData struct {
	TotalItems      int
	TotalCategories int
	TotalPrice      int
}

func HandlePost(w http.ResponseWriter, r *http.Request) {
	// Parse the multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Unable to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create a temporary directory to extract the zip file
	tempDir, err := os.MkdirTemp("", "unzip")
	if err != nil {
		http.Error(w, "Unable to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Save the uploaded file to the temp directory
	zipPath := filepath.Join(tempDir, handler.Filename)
	outFile, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Unable to create file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	// Unzip the file
	unzipDir := filepath.Join(tempDir, "unzipped")
	err = Unzip(zipPath, unzipDir)
	if err != nil {
		http.Error(w, "Unable to unzip file", http.StatusInternalServerError)
		return
	}

	// Read the CSV file
	csvPath := filepath.Join(unzipDir, "data.csv")
	data, err := ReadCSV(csvPath)
	if err != nil {
		http.Error(w, "Unable to read CSV file", http.StatusInternalServerError)
		return
	}

	// Insert data into the database
	insertSQL := `INSERT INTO prices (total_items, total_categories, total_price) VALUES ($1, $2, $3)`
	_, err = db.Exec(insertSQL, data.TotalItems, data.TotalCategories, data.TotalPrice)
	if err != nil {
		http.Error(w, "Unable to insert data into database", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Data successfully uploaded and processed"))
}

func HandleGet(w http.ResponseWriter, r *http.Request) {
	// Query all data from the database
	rows, err := db.Query("SELECT total_items, total_categories, total_price FROM prices")
	if err != nil {
		http.Error(w, "Unable to query data from database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Create a temporary directory to store the CSV file
	tempDir, err := os.MkdirTemp("", "csv")
	if err != nil {
		http.Error(w, "Unable to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Create the CSV file
	csvPath := filepath.Join(tempDir, "data.csv")
	csvFile, err := os.Create(csvPath)
	if err != nil {
		http.Error(w, "Unable to create CSV file", http.StatusInternalServerError)
		return
	}
	defer csvFile.Close()

	// Write CSV header
	csvWriter := csv.NewWriter(csvFile)
	csvWriter.Write([]string{"total_items", "total_categories", "total_price"})

	// Write data rows to the CSV file
	for rows.Next() {
		var totalItems, totalCategories, totalPrice int
		if err := rows.Scan(&totalItems, &totalCategories, &totalPrice); err != nil {
			http.Error(w, "Unable to scan row data", http.StatusInternalServerError)
			return
		}
		csvWriter.Write([]string{
			strconv.Itoa(totalItems),
			strconv.Itoa(totalCategories),
			strconv.Itoa(totalPrice),
		})
	}
	csvWriter.Flush()

	if err := csvWriter.Error(); err != nil {
		http.Error(w, "Error writing CSV data", http.StatusInternalServerError)
		return
	}

	// Create a ZIP archive
	zipPath := filepath.Join(tempDir, "data.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Unable to create ZIP file", http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	// Add the CSV file to the ZIP archive
	csvFileInZip, err := zipWriter.Create("data.csv")
	if err != nil {
		http.Error(w, "Unable to add CSV file to ZIP archive", http.StatusInternalServerError)
		return
	}

	// Copy the CSV file content into the ZIP archive
	fileToZip, err := os.Open(csvPath)
	if err != nil {
		http.Error(w, "Unable to open CSV file for zipping", http.StatusInternalServerError)
		return
	}
	defer fileToZip.Close()

	_, err = io.Copy(csvFileInZip, fileToZip)
	if err != nil {
		http.Error(w, "Unable to copy CSV file to ZIP archive", http.StatusInternalServerError)
		return
	}

	// Serve the ZIP file to the client
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=data.zip")
	http.ServeFile(w, r, zipPath)
}
