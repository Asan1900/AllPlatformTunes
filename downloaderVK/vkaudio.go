package vkapi

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

const headers = "identity"

func getDecryptor(keyURL string) (cipher.StreamReader, error) {
	resp, err := http.Get(keyURL)
	if err != nil {
		return cipher.StreamReader{}, err
	}
	defer resp.Body.Close()

	key, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return cipher.StreamReader{}, err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return cipher.StreamReader{}, err
	}

	return cipher.NewCFBDecrypter(block, make([]byte, aes.BlockSize)), nil
}

func main() {
	outputDir := filepath.Join(".", "output")
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		os.Mkdir(outputDir, os.ModePerm)
	}

	args := os.Args
	if len(args) == 4 {
		_, command, username, password := args[0], args[1], args[2], args[3]
		// Replace this with actual VK API library call
		// vk, err := vkapi.NewVkAPI(username, password, "", "")
		// if err != nil {
		//	fmt.Println("Error:", err)
		//	return
		// }

		if command == "auth" {
			// Simulate token for example purposes
			ioutil.WriteFile("token.txt", []byte("dummy_token"), 0644)
		}
		return
	}

	var token string
	if data, err := ioutil.ReadFile("token.txt"); err == nil {
		token = string(data)
	} else {
		fmt.Println("token.txt not found. Please authorize: ./vkaudio auth <login> <password>")
		return
	}

	// Replace this with actual VK API library call
	// vk, err := vkapi.NewVkAPI("", "", "", token)
	// if err != nil {
	//	fmt.Println("Error:", err)
	//	return
	// }

	dump := false
	dumpFilename := ""
	if len(args) == 3 {
		dump = args[1] == "dump"
		dumpFilename = args[2]
	}

	// Simulate response for example purposes
	resp := map[string]interface{}{
		"catalog": map[string]interface{}{
			"sections": []interface{}{
				map[string]interface{}{
					"id":    "1",
					"title": "Music",
					"url":   "https://example.com",
				},
			},
			"default_section": "1",
		},
		"audios": []interface{}{
			map[string]interface{}{
				"artist": "Artist1",
				"title":  "Title1",
				"url":    "https://example.com/audio.m3u8",
			},
		},
	}

	fmt.Println("Logged as UserID")
	sections := resp["catalog"].(map[string]interface{})["sections"].([]interface{})
	defaultSectionID := resp["catalog"].(map[string]interface{})["default_section"].(string)
	audios := resp["audios"].([]interface{})

	fmt.Printf("Received %d audios\n", len(audios))
	var musicSection map[string]interface{}
	for _, s := range sections {
		section := s.(map[string]interface{})
		if section["id"].(string) == defaultSectionID {
			musicSection = section
			break
		}
	}

	fmt.Printf("Default section: \"%s\": %s: %s\n", musicSection["title"], musicSection["id"], musicSection["url"])
	nextStart := musicSection["next_from"].(string)
	for nextStart != "" {
		// Simulate response for example purposes
		resp = map[string]interface{}{
			"section": map[string]interface{}{
				"next_from": "",
			},
			"audios": []interface{}{
				map[string]interface{}{
					"artist": "Artist2",
					"title":  "Title2",
					"url":    "https://example.com/audio2.m3u8",
				},
			},
		}

		nextStart = resp["section"].(map[string]interface{})["next_from"].(string)
		receivedAudios := resp["audios"].([]interface{})
		audios = append(audios, receivedAudios...)
		fmt.Printf("Received %d audios\n", len(receivedAudios))
	}

	var dumpFile *os.File
	if dump {
		var err error
		dumpFile, err = os.Create(dumpFilename)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer dumpFile.Close()
	}

	for i, track := range audios {
		t := track.(map[string]interface{})
		fmt.Printf("%d. %s — %s\n", i+1, t["artist"], t["title"])
		if dumpFile != nil {
			fmt.Fprintf(dumpFile, "%d. %s — %s\n", i+1, t["artist"], t["title"])
		}

		url, _ := t["url"].(string)
		artist, _ := t["artist"].(string)
		title, _ := t["title"].(string)
		trackName := fmt.Sprintf("%s — %s", artist, title)
		outFileBase := sanitizeFileName(fmt.Sprintf("%d. %s", i+1, trackName))
		outFilePath := filepath.Join(outputDir, outFileBase)
		outFileTS := outFilePath + ".ts"
		outFileMP3 := outFilePath + ".mp3"

		if url != "" {
			if matched, _ := regexp.MatchString(`/\w*\.mp3`, url); matched {
				fmt.Println("Downloading mp3")
				err := downloadFile(url, outFileMP3)
				if err != nil {
					fmt.Println("Error:", err)
				}
			} else if matched, _ := regexp.MatchString(`/\w*\.m3u8`, url); matched {
				baseURL := url[:strings.LastIndex(url, "/")]
				playlist, err := downloadText(url)
				if err != nil {
					fmt.Println("Error:", err)
					continue
				}

				blocks := strings.Split(playlist, "#EXT-X-KEY")
				blocks = blocks[1:] // Skip the first block which is not a segment

				keyURL := extractKeyURL(blocks[0])
				decryptor, err := getDecryptor(keyURL)
				if err != nil {
					fmt.Println("Error:", err)
					continue
				}

				for i, block := range blocks {
					segmentURLs := extractSegmentURLs(block)
					segments := []byte{}
					for _, sURL := range segmentURLs {
						segment, err := downloadFile(baseURL+"/"+sURL, "")
						if err != nil {
							fmt.Println("Error:", err)
							continue
						}
						segments = append(segments, segment...)
					}

					if strings.Contains(block, "METHOD=AES-128") {
						segmentKeyURL := extractKeyURL(block)
						if segmentKeyURL != keyURL {
							keyURL = segmentKeyURL
							decryptor, err = getDecryptor(keyURL)
							if err != nil {
								fmt.Println("Error:", err)
								continue
							}
						}

						decryptedSegments := make([]byte, len(segments))
						decryptor.XORKeyStream(decryptedSegments, segments)
						segments = decryptedSegments
					}

					err = appendToFile(outFileTS, segments)
					if err != nil {
						fmt.Println("Error:", err)
						continue
					}
					fmt.Printf("Processed block %d/%d\n", i+1, len(blocks))
				}

				err = convertToMP3(outFileTS, outFileMP3)
				if err != nil {
					fmt.Println("Error:", err)
				} else {
					fmt.Println("Converted to mp3")
				}
				os.Remove(outFileTS)
			} else {
				fmt.Println("Track unavailable")
			}
		}
	}

	if dumpFile != nil {
		fmt.Printf("Dumped %d tracks to %s\n", len(audios), dumpFilename)
	}
}

func sanitizeFileName(name string) string {
	return strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' {
			return -1
		}
		return r
	}, name)
}

func downloadFile(url, filePath string) ([]byte, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if filePath != "" {
		file, err := os.Create(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return nil, err
		}
		return nil, nil
	}

	return ioutil.ReadAll(resp.Body)
}

func downloadText(url string) (string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	text, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(text), nil
}

func extractKeyURL(block string) string {
	re := regexp.MustCompile(`URI="(.+?)"`)
	match := re.FindStringSubmatch(block)
	if len(match) > 1 {
		return match[1]
	}
	return ""
}

func extractSegmentURLs(block string) []string {
	re := regexp.MustCompile(`URI="(.+?)"`)
	matches := re.FindAllStringSubmatch(block, -1)
	segmentURLs := []string{}
	for _, match := range matches {
		segmentURLs = append(segmentURLs, match[1])
	}
	return segmentURLs
}

func appendToFile(filePath string, data []byte) error {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.Write(data)
	return err
}

func convertToMP3(inputPath, outputPath string) error {
	cmd := exec.Command("ffmpeg", "-i", inputPath, "-q:a", "0", "-map", "a", outputPath)
	return cmd.Run()
}
