package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
)

const Debug = true
const DirectoryFormat = "%s [%s] %s"
const FileFormat = "%s. %s - %s.mp3"

var done = make(chan bool)

func main() {

	if len(os.Args) != 2 {
		fmt.Printf("usage: %s <album url>\n", os.Args[0])
		os.Exit(0)
	}

	artist, album, year, tracks := getTrackData(os.Args[1])
	destinationDirectory := getDestinationDirName(artist, album, year)
	os.Mkdir(destinationDirectory, 0755)

	// Start downloads
	numberOfDownloads := 0
	for i := 0; i < len(tracks); i++ {
		if tracks[i]["file"] != nil {
			numberOfDownloads++
			file := tracks[i]["file"].(map[string]interface{})

			trackNum := fmt.Sprintf("%.0f", tracks[i]["track_num"].(float64))
			track := trackNum
			if tracks[i]["track_num"].(float64) < 10 {
				track = "0" + trackNum
			}

			debugPrint("init go routine " + string(i))
			go downloadTrack(file["mp3-128"].(string), track, tracks[i]["title"].(string), album, artist, year, destinationDirectory)
		}
	}

	// Wait for downloads to finish
	for i := 0; i < numberOfDownloads; i++ {
		<-done
	}
}

func downloadTrack(url string, trackNumber string, title string, album string, artist string, year string, destinationDirectory string) {
	filename := destinationDirectory + string(filepath.Separator) + fmt.Sprintf(FileFormat, trackNumber, artist, title)
	fmt.Println("started downloading: " + filename + " from " + url)

	resp, err := http.Get(url)
	if err != nil {
		panic(err)
	}

	trackUrl := resp.Request.URL.String()

	trackBytes := httpGetBody(trackUrl)

	err = ioutil.WriteFile(filename, trackBytes, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Println("finished downloading: " + filename)

	debugPrint("setting ID3 tag: " + filename)
	cmdArgs := []string{"-t " + title, "-l " + album, "-a " + artist, "-y " + year, "-n " + trackNumber, filename}
	cmd := exec.Command("mp3info", cmdArgs...)
	_, err = cmd.CombinedOutput()
	if err != nil {
		fmt.Println("setting ID3 tag: " + filename + ": " + err.Error())
		os.Exit(1)
	}

	done <- true
}

func getTrackData(url string) (string, string, string, []map[string]interface{}) {
	body := string(httpGetBody(url))

	album := extractSubstring(body, "album_title: \"(.*)\"")
	artist := extractSubstring(body, "artist: \"(.*)\"")
	year := extractSubstring(body, `album_release_date: \".*(\d{4}).*\"`)
	albumInfo := extractSubstring(body, "trackinfo: (.*),")

	jsonData := []byte(albumInfo)

	var tracks = make([]interface{}, 0)

	err := json.Unmarshal(jsonData, &tracks)
	if err != nil {
		panic(err)
	}

	var tracksData []map[string]interface{}
	for i := 0; i < len(tracks); i++ {
		m := tracks[i].(map[string]interface{})
		tracksData = append(tracksData, m)
	}

	return artist, album, year, tracksData
}

func httpGetBody(url string) []byte {
	debugPrint("fetching " + url)

	resp, err := http.Get(url)
	if err != nil {
		errorPrint("failed to fetch " + url)
		os.Exit(1)
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		errorPrint("failed to read body of response")
		os.Exit(1)
	}

	return body[:]
}

func extractSubstring(s string, regex string) string {
	r, _ := regexp.Compile(regex)
	regexResult := r.FindStringSubmatch(s)
	if len(regexResult) != 2 {
		fmt.Printf("error: failed to extract substring (%s)\n", regex)
		os.Exit(1)
	}

	return regexResult[1]
}

func getDestinationDirName(artist string, album string, year string) string {
	return fmt.Sprintf(DirectoryFormat, artist, album, year)
}

func debugPrint(msg string) {
	if Debug {
		fmt.Printf("debug: %s\n", msg)
	}
}

func errorPrint(msg string) {
	fmt.Printf("error: %s\n", msg)
}
