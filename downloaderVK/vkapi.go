package vkapi

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	apiBase      = "https://api.vk.com/method/"
	userAgent    = "VKAndroidApp/6.3-5277 (Android 6.0; SDK 23; armeabi-v7a; ZTE Blade X3; en; 1920x1080)"
	clientID     = "2274003"
	clientSecret = "hHbZxrka2uZ6jB1inYsH"
)

type VkAPIError struct {
	Message string
}

func (e *VkAPIError) Error() string {
	return e.Message
}

type VkAuthError struct {
	Message string
}

func (e *VkAuthError) Error() string {
	return e.Message
}

type VkAPI struct {
	Token         string
	UserID        string
	Version       string
	DeviceID      string
	Authenticated bool
}

func NewVkAPI(login, password, version, token string) (*VkAPI, error) {
	vk := &VkAPI{
		Token:         token,
		Version:       version,
		Authenticated: false,
	}

	if vk.Version == "" {
		vk.Version = "5.123"
	}

	if vk.DeviceID = getDeviceID(); vk.DeviceID == "" {
		vk.DeviceID = generateDeviceID()
		saveDeviceID(vk.DeviceID)
	}

	if vk.Token != "" {
		if err := vk.setUserID(); err != nil {
			return nil, err
		}
	} else {
		if err := vk.tryAuth(login, password, ""); err != nil {
			return nil, err
		}
	}

	return vk, nil
}

func (vk *VkAPI) tryAuth(login, password, code string) error {
	authURL := fmt.Sprintf("https://oauth.vk.com/token?client_id=%s&client_secret=%s&libverify_support=1&scope=all&v=%s&lang=en&device_id=%s&grant_type=password&username=%s&password=%s&2fa_supported=1",
		clientID, clientSecret, vk.Version, vk.DeviceID, url.QueryEscape(login), url.QueryEscape(password))

	if code != "" {
		authURL += "&code=" + code
	}

	resp, err := http.Post(authURL, "application/x-www-form-urlencoded", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	response := parseJSON(body)

	if token, ok := response["access_token"].(string); ok {
		vk.Token = token
		vk.UserID = fmt.Sprintf("%v", response["user_id"])
		vk.Authenticated = true
		fmt.Println("[VK] Authentication succeed")
	} else {
		if response["error_description"] == "use app code" {
			appCode := getAppCode()
			return vk.tryAuth(login, password, appCode)
		} else {
			return &VkAuthError{Message: response["error"].(string)}
		}
	}

	return nil
}

func (vk *VkAPI) request(method string, parameters map[string]string) (map[string]interface{}, error) {
	if parameters == nil {
		parameters = map[string]string{}
	}
	parameters["access_token"] = vk.Token
	parameters["v"] = vk.Version
	parameters["device_id"] = vk.DeviceID
	parameters["lang"] = "en"

	resp, err := http.PostForm(apiBase+method, toURLValues(parameters))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	response := parseJSON(body)

	if err, exists := response["error"]; exists {
		errorMap := err.(map[string]interface{})
		errorCode := int(errorMap["error_code"].(float64))
		errorMsg := errorMap["error_msg"].(string)
		if errorCode == 14 {
			captchaSID := errorMap["captcha_sid"].(string)
			captchaImg := errorMap["captcha_img"].(string)
			if err := handleCaptcha(captchaImg); err != nil {
				return nil, err
			}
			parameters["captcha_sid"] = captchaSID
			parameters["captcha_key"] = getCaptchaKey()
			return vk.request(method, parameters)
		}
		return nil, &VkAPIError{Message: fmt.Sprintf("Error code %d: %s", errorCode, errorMsg)}
	}

	return response["response"].(map[string]interface{}), nil
}

func (vk *VkAPI) upload(url string, file []byte) (string, string, string, error) {
	boundary := generateBoundary()
	headers := map[string]string{
		"user-agent":   userAgent,
		"content-type": "multipart/form-data; boundary=" + boundary,
	}
	filename := randomHex(10) + ".jpg"
	dataHeader := fmt.Sprintf("\r\n--%s\r\nContent-Disposition: form-data; name=\"photo\"; filename=\"%s\"\r\nContent-Type: image/jpeg\r\n\r\n", boundary, filename)
	dataEnd := fmt.Sprintf("\r\n--%s--\r\n", boundary)
	data := append([]byte(dataHeader), file...)
	data = append(data, []byte(dataEnd)...)

	resp, err := http.Post(url, headers["content-type"], bytes.NewReader(data))
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	response := parseJSON(body)

	server, _ := response["server"].(string)
	photo, _ := response["photo"].(string)
	vkHash, _ := response["hash"].(string)

	if server != "" && photo != "" && vkHash != "" {
		return server, photo, vkHash, nil
	}

	return "", "", "", &VkAPIError{Message: "Failed to upload"}
}

// Helper functions

func getAppCode() string {
	fmt.Print("Enter the authentication code: ")
	var code string
	fmt.Scanln(&code)
	return code
}

func handleCaptcha(captchaImg string) error {
	fmt.Println("CAPTCHA required. Please visit the following URL to view the CAPTCHA image:")
	fmt.Println(captchaImg)
	fmt.Print("Enter the CAPTCHA key: ")
	var captchaKey string
	fmt.Scanln(&captchaKey)

	// Store the CAPTCHA key in a temporary place for later use

	// TODO: saveCaptchaKey(captchaKey)
	return nil
}

func getCaptchaKey() string {
	// Declare a variable to store the CAPTCHA key.
	var captchaKey string

	// Prompt the user to enter the CAPTCHA key.
	fmt.Print("Enter the CAPTCHA key: ")

	// Read the user input from the standard input (console).
	fmt.Scanln(&captchaKey)

	// Return the CAPTCHA key.
	return captchaKey
}

func getDeviceID() string {
	if data, err := ioutil.ReadFile(".device_id"); err == nil {
		return string(data)
	}
	return ""
}

func generateDeviceID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func saveDeviceID(deviceID string) {
	ioutil.WriteFile(".device_id", []byte(deviceID), 0644)
}

func (vk *VkAPI) setUserID() error {
	response, err := vk.request("execute.getUserInfo", nil)
	if err != nil {
		return err
	}
	if profile, exists := response["profile"].(map[string]interface{}); exists {
		vk.UserID = fmt.Sprintf("%v", profile["id"])
	}
	return nil
}

func parseJSON(data []byte) map[string]interface{} {
	var result map[string]interface{}
	json.Unmarshal(data, &result)
	return result
}

func toURLValues(params map[string]string) url.Values {
	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	return values
}

func generateBoundary() string {
	b := make([]byte, 30)
	rand.Read(b)
	return "----WebKitFormBoundary" + hex.EncodeToString(b)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Handle CAPTCHA and get App Code functions need to be implemented
