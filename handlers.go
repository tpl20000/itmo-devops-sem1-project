package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
	"bytes"
)

type PriceData struct {
	Name        string  `json:"name"`
	Category    string  `json:"category"`
	Price       float64 `json:"price"`
	Create_date time.Time
}

// HandlePost processes a POST request to upload and process a file
func HandlePost(w http.ResponseWriter, r *http.Request) {
	// Parse the multipart form
	err := r.ParseMultipartForm(10 << 20) // 10 MB
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("file")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to retrieve file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Create a temporary directory to extract the ZIP file
	tempDir, err := os.MkdirTemp("", "unzip")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create temp directory", http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	// Save the uploaded file to the temp directory
	zipPath := filepath.Join(tempDir, handler.Filename)
	outFile, err := os.Create(zipPath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create file", http.StatusInternalServerError)
		return
	}
	defer outFile.Close()

	_, err = io.Copy(outFile, file)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	// Unzip the file
	err = Unzip(zipPath, tempDir)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to unzip file", http.StatusInternalServerError)
		return
	}

	// Find the first CSV file in the temp directory
	csvFile, err := findFirstCSVFile(tempDir)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to find CSV file", http.StatusInternalServerError)
		return
	}

	// Read the CSV file
	data, err := ReadCSV(csvFile)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to read CSV file", http.StatusInternalServerError)
		return
	}

	// Insert data into the database
	tx, err := db.Begin()
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to begin transaction", http.StatusInternalServerError)
		return
	}

	stmt, err := tx.Prepare("INSERT INTO prices (product_name, product_category, product_price, manufacture_date) VALUES ($1, $2, $3, $4)")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to prepare statement", http.StatusInternalServerError)
		return
	}
	defer stmt.Close()

	var totalItems int
	var totalCategories int
	var totalPrice float64

	for _, priceData := range data {
		_, err = stmt.Exec(priceData.Name, priceData.Category, priceData.Price, priceData.Create_date)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Unable to execute statement", http.StatusInternalServerError)
			return
		}

		totalItems++
		totalPrice += priceData.Price

		// Check if category is already counted
		var count int
		err = db.QueryRow("SELECT COUNT(*) FROM prices WHERE product_category = $1", priceData.Category).Scan(&count)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Unable to query database", http.StatusInternalServerError)
			return
		}

		if count == 1 {
			totalCategories++
		}
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to commit transaction", http.StatusInternalServerError)
		return
	}

	// Get total categories and total price from database
	var totalCategoriesDB int
	var totalPriceDB float64
	err = db.QueryRow("SELECT COUNT(DISTINCT product_category), SUM(product_price) FROM prices").Scan(&totalCategoriesDB, &totalPriceDB)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to query database", http.StatusInternalServerError)
		return
	}

	// Create a response struct
	type Response struct {
		ItemsAdded      int     `json:"total_items"`
		TotalCategories int     `json:"total_categories"`
		TotalPrice      float64 `json:"total_price"`
	}

	// Marshal the response to JSON
	response := Response{
		ItemsAdded:      totalItems,
		TotalCategories: totalCategoriesDB,
		TotalPrice:      totalPriceDB,
	}

	// Convert the response to a byte array
	jsonData, err := json.Marshal(response)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to marshal data to JSON", http.StatusInternalServerError)
		return
	}

	fmt.Println("POST handled! Responce: ", string(jsonData))

	// Write the JSON data to the response writer
	w.Header().Set("Content-Type", "application/json")
	w.Write(jsonData)

}

// HandleGet processes a GET request to retrieve all data from the database
func HandleGet(w http.ResponseWriter, r *http.Request) {
	// Query the database to retrieve all data
	rows, err := db.Query("SELECT id, product_name, product_category, product_price, manufacture_date FROM prices")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to query database", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	// Create a temporary directory to store the CSV file
	tempDir, err := os.MkdirTemp(".", "temp")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create temp directory", http.StatusInternalServerError)
		return
	}
	//defer os.RemoveAll(tempDir)

	// Create a CSV file in the temporary directory
	csvPath := filepath.Join(tempDir, "data.csv")
	csvFile, err := os.Create(csvPath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create CSV file", http.StatusInternalServerError)
		return
	}
	defer csvFile.Close()

	// Write the data to the CSV file
	csvWriter := csv.NewWriter(csvFile)

	// Write the header row
	err = csvWriter.Write([]string{"id", "product_name", "product_category", "product_price", "manufacture_date"})
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to write CSV header", http.StatusInternalServerError)
		return
	}

	// Write each row of data to the CSV file
	for rows.Next() {
		var (
			id         int
			name       string
			category   string
			price      float64
			createDate time.Time
		)
		err = rows.Scan(&id, &name, &category, &price, &createDate)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Unable to scan row", http.StatusInternalServerError)
			return
		}

		err = csvWriter.Write([]string{strconv.Itoa(id), name, category, fmt.Sprintf("%.2f", price), createDate.Format("2006-01-02")})
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Unable to write CSV row", http.StatusInternalServerError)
			return
		}
	}

	csvWriter.Flush()

	// Zip the CSV file
	zipPath := filepath.Join(tempDir, "response.zip")
	zipFile, err := os.Create(zipPath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create ZIP file", http.StatusInternalServerError)
		return
	}
	defer zipFile.Close()

	zipWriter := zip.NewWriter(zipFile)
	defer zipWriter.Close()

	content, err := os.ReadFile(csvPath)
	fmt.Println("File contains: ", string(content))

	// Create a new file in the ZIP archive
	archiveWriter, err := zipWriter.Create("data.csv")
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create file in ZIP archive", http.StatusInternalServerError)
		return
	}

	archiveWriter.Write(content)
	zipWriter.Flush()

	archiveReader, err := os.ReadFile(zipPath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to read file in ZIP archive", http.StatusInternalServerError)
		return
	}

	// Return the ZIP file as the response
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=response.zip")
	w.WriteHeader(http.StatusOK)

	fmt.Println("Serving file: ", zipPath)
	fmt.Println("Sending ", len(archiveReader), " bytes")

	w.Write(archiveReader)

	//http.ServeFile(w, r, zipPath)
	fmt.Println("GET handled!")
}
