package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

	// Insert data into the database and calculate total categories and price
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
	var totalPrice float64
	var categories []string

	for _, priceData := range data {
		_, err = stmt.Exec(priceData.Name, priceData.Category, priceData.Price, priceData.Create_date)
		if err != nil {
			fmt.Println(err)
			http.Error(w, "Unable to execute statement", http.StatusInternalServerError)
			return
		}

		totalItems++
		totalPrice += priceData.Price
		categories = append(categories, priceData.Category)
	}

	err = tx.Commit()
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to commit transaction", http.StatusInternalServerError)
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
		TotalCategories: len(removeDuplicates(categories)),
		TotalPrice:      totalPrice,
	}

	// Convert the response to a byte array
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to parse JSON", http.StatusInternalServerError)
	}
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
	defer os.RemoveAll(tempDir)

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

	zipByteBuf, err := Zip(csvPath)
	if err != nil {
		fmt.Println(err)
		http.Error(w, "Unable to create zip file", http.StatusInternalServerError)
	}

	// Return the ZIP file as the response
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", "attachment; filename=response.zip")
	w.WriteHeader(http.StatusOK)

	fmt.Println("Sending ", len(zipByteBuf), " bytes")

	w.Write(zipByteBuf)
}
