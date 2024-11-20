// https://github.com/cloud-barista/cb-tumblebug/discussions/1773
// Credential 등록 관련 APIs
//
//	GET /credential/publicKey
//	POST /credential
//
// CSP별 Credential 등록 포멧
// https://github.com/cloud-barista/cb-spider/wiki/features-and-usages
//   AWS 예시 : curl -sX GET http://localhost:1024/spider/cloudos/metainfo/AWS -H 'Content-Type: application/json' |json_pp |more
//		- 응답 값중 아래 둘 중 하나의 형태로 입력
//		    - Credential : cb-spider 형식
//			- CredentialCSP : CSP 형식
//  [최종] curl -sX GET http://localhost:1024/spider/cloudos/metainfo/aws -H 'Content-Type: application/json' | jq '.Credential'
//         curl -sX POST http://localhost:1323/tumblebug/forward/cloudos/metainfo/aws -u default:default  -d '{}'
//		   curl -sX GET http://localhost:1323/tumblebug/credential/publicKey -u default:default

package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"
)

const (
	AVAILABLE_CSP_LIST_URL      = "/provider"
	GET_CSP_CREDENTIAL_META_URL = "/forward/cloudos/metainfo/"

	GET_PUBLICKEY_URL   = "/credential/publicKey"
	POST_CREDENTIAL_URL = "/credential"
)

var client = resty.New()
var req = client.R()

var host string
var port string
var isInit bool
var csp string

var configFile string
var headers []string

var username string
var password string
var authToken string
var isVerbose bool

// var inputFileData string
// var sendData string

type ServiceInfo struct {
	BaseURL      string `yaml:"baseurl"`
	Auth         Auth   `yaml:"auth"`
	ResourcePath string `yaml:"resourcePath"`
	Method       string `yaml:"method"`
}

// basic : username / password
// bearer : token
type Auth struct {
	Type     string `yaml:"type"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	Token    string `yaml:"token,omitempty"`
}

var serviceInfo ServiceInfo

func SetBasicAuth() {
	// Set basic authentication
	if serviceInfo.Auth.Username != "" && serviceInfo.Auth.Password != "" {
		if isVerbose {
			fmt.Println("setting basic auth")
			fmt.Println("username : " + serviceInfo.Auth.Username)
			fmt.Println("password : " + serviceInfo.Auth.Password)
		}
		client.SetBasicAuth(serviceInfo.Auth.Username, serviceInfo.Auth.Password)
	}
}

// 인증 처리
func SetAuth() {
	switch strings.ToLower(serviceInfo.Auth.Type) {
	case "none", "":
		// 인증이 필요 없는 경우 아무 것도 하지 않음
	case "basic":
		// Set basic authentication
		SetBasicAuth()
	case "bearer":
		// Set Bearer authentication
		if serviceInfo.Auth.Token != "" {
			if isVerbose {
				fmt.Println("Setting bearer auth")
				fmt.Println("Token : " + serviceInfo.Auth.Token)
			}
			client.SetAuthToken(serviceInfo.Auth.Token)
		}
	default:
		SetBasicAuth() // Set basic authentication
		//fmt.Println("Unknown authentication type:", serviceInfo.Auth.Type)
	}
}

// Tumblebug의 정보를 확인 함.
func checkServiceInfo() error {
	fmt.Printf("Configuration file[%s] processing...\n", configFile)
	serviceName := "cb-tumblebug"
	// 서비스 검증
	if !viper.IsSet("services." + serviceName) {
		return errors.New("the name of the service[" + serviceName + "] you want to call is not on the list of supported services.\nPlease check the api.yaml configuration file or the list of available services")
	}

	// 서비스 정보 파싱
	err := viper.UnmarshalKey("services."+serviceName, &serviceInfo)
	if err != nil {
		return err
	}

	// fmt.Printf("Service Info: %+v\n", serviceInfo)

	// // CLI로 인증 정보를 전달 받았을 경우 인증 처리
	// 인증 정보를 CLI로 전달 받았을 경우 처리
	if authToken != "" {
		serviceInfo.Auth.Token = authToken
	}
	if username != "" {
		serviceInfo.Auth.Username = username
	}
	if password != "" {
		serviceInfo.Auth.Password = password
	}

	if serviceInfo.BaseURL == "" {
		return errors.New("couldn't find the BaseURL information for the service to call\nPlease check the api.yaml configuration file")
	}

	// 사용자 입력 host, port로 BaseURL 정보 업데이트
	if host != "" || port != "" {
		updateBaseURL(&serviceInfo.BaseURL, host, port)
	}

	SetAuth()

	fmt.Printf("Configuration file[%s] processed.\n", configFile)
	fmt.Printf("Tumblebug Base URL : %s\n", serviceInfo.BaseURL)
	return nil
}

func updateBaseURL(baseURL *string, host string, port string) error {
	// baseURL을 파싱
	parsedURL, err := url.Parse(*baseURL)
	if err != nil {
		return err
	}

	// host 값이 제공된 경우 parsedURL의 Hostname을 새로운 host 값으로 변경
	if host != "" {
		if port != "" {
			parsedURL.Host = host + ":" + port
		} else {
			parsedURL.Host = host + ":" + parsedURL.Port()
		}
	} else if port != "" {
		// host 값이 제공되지 않고 port만 제공된 경우 기존 hostname에 새로운 port를 설정
		parsedURL.Host = parsedURL.Hostname() + ":" + port
	}

	// 업데이트된 URL을 문자열로 변환하여 baseURL에 반영
	*baseURL = parsedURL.String()
	return nil
}

// Tumblebug으로부터 사용 가능한 CSP 목록을 가져옴.
func getCspList() ([]string, error) {
	url := serviceInfo.BaseURL + AVAILABLE_CSP_LIST_URL
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	//resp, err := client2.R().Get(url)
	resp, err := req.Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	if isVerbose {
		fmt.Println(string(resp.Body()))
	}

	// JSON 결과 값에서 "output" 값을 추출
	var result map[string]interface{}
	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	if output, ok := result["output"].([]interface{}); ok {
		// output 값을 문자열 배열로 변환
		outputArray := make([]string, len(output))
		for i, v := range output {
			outputArray[i] = fmt.Sprintf("%v", v)
		}
		// 알파벳 순으로 정렬
		sort.Strings(outputArray)
		return outputArray, nil
	} else {
		return nil, fmt.Errorf("Output key not found or is not an array in response")
	}
}

// func selectCspFromCLI(cspList []string) (string, error) {
func selectCspFromCLI() (string, error) {
	// CSP 목록을 가져옴
	cspList, err := getCspList()
	if err != nil {
		fmt.Println("Error:", err)
		return "", fmt.Errorf("Error: %v", err)
	}

	// cspList 배열의 값이 없을 경우 에러를 반환
	if len(cspList) == 0 {
		return "", fmt.Errorf("No available CSPs found")
	}

	// CSP 목록을 출력
	fmt.Println("Available CSPs:")
	for i, csp := range cspList {
		fmt.Printf("%d. %s\n", i+1, csp)
	}
	fmt.Println("0. Exit")

	// 사용자로부터 CSP 번호를 입력받음
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Please select a CSP by number: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("Error reading input: %v", err)
		}
		input = strings.TrimSpace(input)

		// 입력된 값을 정수로 변환
		selection, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}

		// 0을 입력한 경우 종료
		if selection == 0 {
			//fmt.Println("Exiting.")
			//return "", nil
			return "", fmt.Errorf("No CSP selected. Exiting.")
		}

		// 유효한 번호인지 확인
		if selection > 0 && selection <= len(cspList) {
			// 선택된 CSP 값을 소문자로 변환하여 저장
			return strings.ToLower(cspList[selection-1]), nil
			//return cspList[selection-1], nil
		} else {
			fmt.Println("Invalid selection. Please try again.")
		}
	}
}

// 사용자로부터 콘솔에서 CSP입력을 받아 처리
func getCredentialsMeta(csp string) ([]string, error) {
	// csp에 대한 인증 정보 입력 형식을 조회합니다.
	fmt.Printf("Retrieving credential input format for %s\n", csp)

	// curl -sX POST http://localhost:1323/tumblebug/forward/cloudos/metainfo/aws -u default:default  -d '{}'
	/*
		credentials := map[string]string{
			"ClientId":     "your-client-id",
			"ClientSecret": "your-client-secret",
		}
	*/

	url := serviceInfo.BaseURL + GET_CSP_CREDENTIAL_META_URL + csp
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	req.SetBody("{}")

	resp, err := req.Post(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	if isVerbose {
		fmt.Println(string(resp.Body()))
	}

	// JSON 결과 값에서 "Credential" 값을 추출
	var result map[string]interface{}
	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	if credential, ok := result["Credential"].([]interface{}); ok {
		fmt.Printf("Successfully retrieved credential meta information for %s\n", csp)

		// Credential 값을 문자열 배열로 변환
		credentialArray := make([]string, len(credential))
		for i, v := range credential {
			credentialArray[i] = fmt.Sprintf("%v", v)
		}

		return credentialArray, nil
	} else {
		return nil, fmt.Errorf("Credential key not found in response")
	}

}

// CSP의 인증 정보를 암호화 처리 함.
func processCspCredentialEncrypt() {
	selectedCsp := ""

	// CLI에서 옵션으로 전달 받은 경우
	if csp != "" {
		selectedCsp = strings.ToLower(csp)
	} else {
		var err error
		// 사용자로부터 콘솔에서 CSP를 선택받음
		selectedCsp, err = selectCspFromCLI()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}

	// selectedCsp 값을 소문자로 변환하여 저장
	//selectedCsp = strings.ToLower(selectedCsp)

	// 선택된 CSP에 대한 인증 정보를 처리한다는 메시지 출력
	fmt.Printf("\nProcessing authentication information for selected [%s] CSP\n", selectedCsp)

	// CSP에 맞는 Credential 정보를 가져옴
	credentialMeta, err := getCredentialsMeta(selectedCsp)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// 상세 모드에서는 입력 받아야할 Credential 키 값 출력
	if isVerbose {
		// fmt.Println("Credential Meta :", credentialMeta)
		fmt.Println("The following credential information is required:")
		for _, key := range credentialMeta {
			fmt.Println(key)
		}
		fmt.Println()
	}

	// 사용자로부터 Credential 값을 입력받음
	credentials, err := inputCredentialsFromCli(credentialMeta)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// PublicKey와 PublicKeyTokenId를 가져옴
	publicKeyResponse, err := getPublicKey()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if isVerbose {
		// PublicKey와 PublicKeyTokenId 값을 출력
		fmt.Println("PublicKeyTokenId:", publicKeyResponse.PublicKeyTokenId)
		fmt.Println("PublicKey:", publicKeyResponse.PublicKey)
	}

	// Credential 정보를 PublicKey로 암호화 처리
	encryptedCredentials, encryptedAesKey := encryptCredentialsWithPublicKey(publicKeyResponse.PublicKey, credentials)
	if isVerbose {
		fmt.Println("Encrypted Credentials:", encryptedCredentials)
		fmt.Println("Encrypted AES Key:", encryptedAesKey)
	}

	// 암호화된 Credential 정보를 서버로 전송
	payload := map[string]interface{}{
		"credentialHolder":                 "admin",
		"providerName":                     selectedCsp,
		"publicKeyTokenId":                 publicKeyResponse.PublicKeyTokenId,
		"encryptedClientAesKeyByPublicKey": encryptedAesKey,
		"credentialKeyValueList":           encryptedCredentials,
	}
	sendCredentials(payload)
}

// 사용자로부터 콘솔에서 CSP의 인증 정보를 입력받아 map 형태로 반환
func inputCredentialsFromCli(credentialMeta []string) (map[string]string, error) {
	credentials := make(map[string]string)
	reader := bufio.NewReader(os.Stdin)

	for {
		// ================================
		// CSP 인증 정보를 입력 받음
		// ================================
		for _, key := range credentialMeta {
			fmt.Printf("Please enter %s: ", key)
			// value, err := reader.ReadString('\n')
			// 입력을 숨김
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return nil, fmt.Errorf("Error reading input: %v", err)
			}
			value := string(bytePassword)
			fmt.Println() // 줄바꿈
			credentials[key] = strings.TrimSpace(value)
		}

		// ================================
		// 입력된 값을 확인할 것인지 물어봄
		// ================================
		for {
			fmt.Print("Do you want to review the entered credentials? (yes/no): ")
			review, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("Error reading input: %v", err)
			}
			review = strings.TrimSpace(strings.ToLower(review))

			if review == "yes" {
				// 입력된 값들을 출력하여 확인
				fmt.Println("You have entered the following credentials:")
				for key, value := range credentials {
					fmt.Printf("%s: %s\n", key, value)
				}
				break
			} else if review == "no" {
				break
			} else {
				fmt.Println("Invalid input. Please enter 'yes' or 'no'.")
			}
		}

		// ================================
		// 입력된 값을 확인할 것인지 물어봄
		// ================================
		fmt.Print("Is this correct? (yes/no/retry): ")
		confirmation, err := reader.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("Error reading input: %v", err)
		}
		confirmation = strings.TrimSpace(strings.ToLower(confirmation))

		if confirmation == "yes" {
			return credentials, nil
		} else if confirmation == "no" {
			return nil, fmt.Errorf("User cancelled input")
		} else if confirmation == "retry" {
			credentials = make(map[string]string)
			fmt.Println("Please re-enter the credentials.")
		} else {
			fmt.Println("Invalid input. Please enter 'yes', 'no', or 'retry'.")
		}
	}
}

// PublicKeyResponse 구조체 정의
type PublicKeyResponse struct {
	PublicKeyTokenId string `json:"publicKeyTokenId"`
	PublicKey        string `json:"publicKey"`
}

// 서버로부터 PublicKey와 PublicKeyTokenId를 조회 함.
func getPublicKey() (*PublicKeyResponse, error) {
	fmt.Println("Retrieving public key and public key token id")
	url := serviceInfo.BaseURL + GET_PUBLICKEY_URL
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	resp, err := req.Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	if isVerbose {
		fmt.Println(string(resp.Body()))
	}

	// JSON 결과 값을 PublicKeyResponse 구조체로 변환
	var publicKeyResponse PublicKeyResponse
	err = json.Unmarshal(resp.Body(), &publicKeyResponse)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	return &publicKeyResponse, nil
}

func encryptCredentialsWithPublicKey(publicKeyPem string, credentials map[string]string) (map[string]string, string) {
	// Parse public key from PEM format
	block, _ := pem.Decode([]byte(publicKeyPem))
	if block == nil {
		panic("Failed to decode PEM block containing public key")
	}

	rsaPublicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}

	// Generate AES key
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
		panic(err)
	}

	// Encrypt credentials using AES
	encryptedCredentials := make(map[string]string)
	for k, v := range credentials {
		aesCipher, err := aes.NewCipher(aesKey)
		if err != nil {
			panic(err)
		}

		gcm, err := cipher.NewGCM(aesCipher)
		if err != nil {
			panic(err)
		}

		nonce := make([]byte, gcm.NonceSize())
		if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
			panic(err)
		}

		ciphertext := gcm.Seal(nonce, nonce, []byte(v), nil)
		encryptedCredentials[k] = base64.StdEncoding.EncodeToString(ciphertext)
	}

	// Encrypt AES key using RSA public key
	encryptedAesKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPublicKey, aesKey, nil)
	if err != nil {
		panic(err)
	}

	return encryptedCredentials, base64.StdEncoding.EncodeToString(encryptedAesKey)
}

// 서버로 암호화된 Credential 정보를 전송
func sendCredentials(payload map[string]interface{}) (map[string]interface{}, error) {
	fmt.Println("Sending encrypted credentials to server")

	// POST_CREDENTIAL_URL = "/credential"
	/*
		client := &http.Client{}
		reqBody, _ := json.Marshal(payload)
		req, err := http.NewRequest("POST", "http://localhost:1323/tumblebug/credential", bytes.NewBuffer(reqBody))
		if err != nil {
			panic(err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Basic ...")

		resp, err := client.Do(req)
		if err != nil {
			panic(err)
		}
		defer resp.Body.Close()

		body, _ := io.ReadAll(resp.Body)
		fmt.Println(string(body))
	*/

	if isVerbose {
		fmt.Println("Request payload")
		// Print the payload in a pretty-printed JSON format
		payloadJSON, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			panic(err)
		}

		fmt.Println("Payload:")
		fmt.Println(string(payloadJSON)) // 출력
	}

	url := serviceInfo.BaseURL + POST_CREDENTIAL_URL
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	resp, err := req.SetBody(payload).Post(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	defer resp.RawBody().Close()

	body, err := io.ReadAll(resp.RawBody())
	if err != nil {
		return nil, fmt.Errorf("Error reading response body: %v", err)
	}

	if isVerbose {
		fmt.Println(string(body))
	}

	// 응답 결과를 처리하여 리턴 타입에 맞게 반환
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	return result, nil
}

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "Registration of CSP-Specific Credentials and Default Resources",
	Long: `Supports the registration of CSP credentials and initial data
	The basic information of the subsystem is utilized from the api.yaml file, but the user can change the API authentication information including host and port.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		isInit = false

		// 서브 커맨드가 입력되었을 때에는 도움말을 출력하지 않음.
		if len(args) == 0 && cmd.Flags().NFlag() == 0 && cmd.HasSubCommands() {
			fmt.Println(cmd.Help())
			return
		}

		// 설정 파일 처리
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Printf("Error reading config file: %s\n", err)
			return
		}

		// 호출할 서비스 정보 처리
		errParse := checkServiceInfo()
		if errParse != nil {
			fmt.Println(errParse)
			return
		}

		isInit = true

		//fmt.Println("cliSpecVersion : ", viper.GetString("cliSpecVersion"))
		//fmt.Println("Loaded configurations:", viper.AllSettings())
	},

	Run: func(cmd *cobra.Command, args []string) {
		if !isInit {
			return
		}

		publicKeyResponse, err := getPublicKey()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}

		if isVerbose {
			// PublicKey와 PublicKeyTokenId 값을 출력
			fmt.Println("PublicKeyTokenId:", publicKeyResponse.PublicKeyTokenId)
			fmt.Println("PublicKey:", publicKeyResponse.PublicKey)
		}

		// CSP의 인증 정보를 암호화 처리
		//processCspCredentialEncrypt()
	},
}

func init() {
	setupCmd.AddCommand(credentialCmd)
	credentialCmd.Flags().StringVarP(&configFile, "config", "c", common.API_FILE, "config file")

	credentialCmd.Flags().StringVarP(&host, "host", "", "localhost", "The server address where Tumblebug is running (Default: localhost)")
	credentialCmd.Flags().StringVarP(&port, "port", "", "1323", "The port number Tumblebug is using (Default: 1323)")
	credentialCmd.Flags().StringVarP(&csp, "csp", "", "", "The cloud service provider (CSP) to register")

	// Add flags for headers
	credentialCmd.Flags().StringSliceVarP(&headers, "header", "H", []string{}, "Pass custom header(s) to server")

	// // Add flags for basic authentication
	credentialCmd.Flags().StringVarP(&authToken, "authToken", "", "", "sets the auth token of the 'Authorization' header for all HTTP requests.(The default auth scheme is 'Bearer')")
	credentialCmd.Flags().StringVarP(&username, "user", "u", "", "Username for basic authentication") // - sets the basic authentication header in the HTTP request
	credentialCmd.Flags().StringVarP(&password, "password", "p", "", "Password for basic authentication")

	// credentialCmd.Flags().StringVarP(&inputFileData, "file", "f", "", "Data to send to the server from file")

	credentialCmd.Flags().BoolVarP(&isVerbose, "verbose", "v", false, "Show more detail information")

}
