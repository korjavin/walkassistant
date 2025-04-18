package main

import (
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"

	"github.com/tkrajina/gpxgo/gpx"
)

func main() {
	http.HandleFunc("/upload", uploadHandler)
	fmt.Println("Starting server at port 8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the multipart form
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		http.Error(w, "Unable to parse form", http.StatusBadRequest)
		return
	}

	// Get the file from the form
	file, handler, err := r.FormFile("gpxfile")
	if err != nil {
		http.Error(w, "Unable to get file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// Save the file to the data directory
	err = saveFile(file, handler.Filename)
	if err != nil {
		http.Error(w, "Unable to save file", http.StatusInternalServerError)
		return
	}

	// Parse the GPX file
	gpxData, err := parseGPX(handler.Filename)
	if err != nil {
		http.Error(w, "Unable to parse GPX file", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "File uploaded and parsed successfully: %s", handler.Filename)

	// Analyze routes
	tracks, err := analyzeRoutes(gpxData)
	if err != nil {
		http.Error(w, "Unable to analyze routes", http.StatusInternalServerError)
		return
	}

	// For now, just print the number of tracks found
	fmt.Fprintf(w, "Number of tracks found: %d", len(tracks))
}

func saveFile(file multipart.File, filename string) error {
	// Create the data directory if it doesn't exist
	err := os.MkdirAll("data", os.ModePerm)
	if err != nil {
		return err
	}

	// Create the file in the data directory
	dst, err := os.Create(fmt.Sprintf("data/%s", filename))
	if err != nil {
		return err
	}
	defer dst.Close()

	// Copy the uploaded file to the destination file
	_, err = io.Copy(dst, file)
	if err != nil {
		return err
	}

	return nil
}

func parseGPX(filename string) (*gpx.GPX, error) {
	filePath := fmt.Sprintf("data/%s", filename)
	gpxFile, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer gpxFile.Close()

	gpxData, err := gpx.Parse(gpxFile)
	if err != nil {
		return nil, err
	}

	return gpxData, nil
}

func analyzeRoutes(gpxData *gpx.GPX) ([]gpx.GPXTrack, error) {
	// Placeholder for route analysis logic
	// For now, we'll just return the existing tracks
	return gpxData.Tracks, nil
}
