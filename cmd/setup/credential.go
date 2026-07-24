// https://github.com/cloud-barista/cb-tumblebug/discussions/1773
// APIs related to credential registration
//
//	GET /credential/publicKey
//	POST /credential
//
// Credential registration format per CSP
// https://github.com/cloud-barista/cb-spider/wiki/features-and-usages
//   AWS example : curl -sX GET http://localhost:1024/spider/cloudos/metainfo/AWS -H 'Content-Type: application/json' |json_pp |more
//		- From the response, provide one of the two forms below
//		    - Credential : cb-spider format
//			- CredentialCSP : CSP format
//  [final] curl -sX GET http://localhost:1024/spider/cloudos/metainfo/aws -H 'Content-Type: application/json' | jq '.Credential'
//         curl -sX POST http://localhost:1323/tumblebug/forward/cloudos/metainfo/aws -u default:default  -d '{}'
//		   curl -sX GET http://localhost:1323/tumblebug/credential/publicKey -u default:default

package setup

import (
	"bufio"
	"bytes"
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

	GET_PUBLICKEY_URL = "/credential/publicKey"
	// #nosec G101 -- a REST path on the tumblebug API, not a credential value
	POST_CREDENTIAL_URL = "/credential"
)

var client = common.NewHTTPClient()

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

// Applies the headers the user passed with -H to the request
func setHeaders(r *resty.Request) *resty.Request {
	for _, h := range headers {
		headerParts := strings.SplitN(h, ":", 2)
		if len(headerParts) != 2 {
			fmt.Println("Invalid header format:", h)
			continue
		}
		r.Header.Set(strings.TrimSpace(headerParts[0]), strings.TrimSpace(headerParts[1]))
		if isVerbose {
			fmt.Printf("%s : %s\n", strings.TrimSpace(headerParts[0]), strings.TrimSpace(headerParts[1]))
		}
	}
	return r
}

// newRequest builds a new request on every call.
// Reusing a single request would leak state set by an earlier call, such as the body, into the next one.
func newRequest() *resty.Request {
	return setHeaders(client.R())
}

func SetBasicAuth() {
	// Set basic authentication
	if serviceInfo.Auth.Username != "" && serviceInfo.Auth.Password != "" {
		if isVerbose {
			fmt.Println("setting basic auth")
			fmt.Println("username : " + serviceInfo.Auth.Username)
			// Masked: -v here is about confirming which credential was picked
			// up, which a prefix answers without writing the password itself
			// into the terminal history.
			fmt.Println("password : " + common.MaskSecret(serviceInfo.Auth.Password))
		}
		client.SetBasicAuth(serviceInfo.Auth.Username, serviceInfo.Auth.Password)
	}
}

// Sets up authentication
func SetAuth() {
	switch strings.ToLower(serviceInfo.Auth.Type) {
	case "none", "":
		// nothing to do when no authentication is required
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

// Checks the Tumblebug service information.
func checkServiceInfo() error {
	fmt.Printf("Configuration file[%s] processing...\n", configFile)
	serviceName := "cb-tumblebug"
	// Verify the service
	if !viper.IsSet("services." + serviceName) {
		return errors.New("the name of the service[" + serviceName + "] you want to call is not on the list of supported services.\nPlease check the api.yaml configuration file or the list of available services")
	}

	// Parse the service information
	err := viper.UnmarshalKey("services."+serviceName, &serviceInfo)
	if err != nil {
		return err
	}

	// fmt.Printf("Service Info: %+v\n", serviceInfo)

	// // Handle authentication when the credentials were given on the CLI
	// Apply the credentials given on the CLI
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

	// Update the BaseURL with the host and port entered by the user
	// The override only fails when the configured BaseURL cannot be parsed. That
	// used to pass unnoticed and the request went to the unmodified address, so
	// warn instead — the printed BaseURL below then explains where it really went.
	if host != "" || port != "" {
		if err := updateBaseURL(&serviceInfo.BaseURL, host, port); err != nil {
			fmt.Printf("Warning: could not apply the host/port override to %s: %v\n", serviceInfo.BaseURL, err)
		}
	}

	SetAuth()

	fmt.Printf("Configuration file[%s] processed.\n", configFile)
	fmt.Printf("Tumblebug Base URL : %s\n", serviceInfo.BaseURL)
	return nil
}

func updateBaseURL(baseURL *string, host string, port string) error {
	// Parse the baseURL
	parsedURL, err := url.Parse(*baseURL)
	if err != nil {
		return err
	}

	// When a host is given, replace the hostname of parsedURL with that host
	if host != "" {
		if port != "" {
			parsedURL.Host = host + ":" + port
		} else {
			parsedURL.Host = host + ":" + parsedURL.Port()
		}
	} else if port != "" {
		// When only a port is given, keep the existing hostname and apply the new port
		parsedURL.Host = parsedURL.Hostname() + ":" + port
	}

	// Write the updated URL back into baseURL as a string
	*baseURL = parsedURL.String()
	return nil
}

// Retrieves the list of available CSPs from Tumblebug.
func getCspList() ([]string, error) {
	url := serviceInfo.BaseURL + AVAILABLE_CSP_LIST_URL
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	//resp, err := client2.R().Get(url)
	resp, err := newRequest().Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	if isVerbose {
		fmt.Println(string(resp.Body()))
	}

	// Extract the "output" value out of the JSON result
	var result map[string]interface{}
	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	if output, ok := result["output"].([]interface{}); ok {
		// Convert the output value into a string array
		outputArray := make([]string, len(output))
		for i, v := range output {
			outputArray[i] = fmt.Sprintf("%v", v)
		}
		// Sort alphabetically
		sort.Strings(outputArray)
		return outputArray, nil
	} else {
		return nil, fmt.Errorf("Output key not found or is not an array in response")
	}
}

// func selectCspFromCLI(cspList []string) (string, error) {
func selectCspFromCLI() (string, error) {
	// Get the CSP list
	cspList, err := getCspList()
	if err != nil {
		fmt.Println("Error:", err)
		return "", fmt.Errorf("Error: %v", err)
	}

	// Return an error when the cspList array is empty
	if len(cspList) == 0 {
		return "", fmt.Errorf("No available CSPs found")
	}

	// Print the CSP list
	fmt.Println("Available CSPs:")
	for i, csp := range cspList {
		fmt.Printf("%d. %s\n", i+1, csp)
	}
	fmt.Println("0. Exit")

	// Read the CSP number from the user
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Please select a CSP by number: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("Error reading input: %v", err)
		}
		input = strings.TrimSpace(input)

		// Convert the entered value to an integer
		selection, err := strconv.Atoi(input)
		if err != nil {
			fmt.Println("Invalid input. Please enter a number.")
			continue
		}

		// Exit when 0 was entered
		if selection == 0 {
			//fmt.Println("Exiting.")
			//return "", nil
			return "", fmt.Errorf("No CSP selected. Exiting.")
		}

		// Check that the number is valid
		if selection > 0 && selection <= len(cspList) {
			// Return the selected CSP lowercased
			return strings.ToLower(cspList[selection-1]), nil
			//return cspList[selection-1], nil
		} else {
			fmt.Println("Invalid selection. Please try again.")
		}
	}
}

// Takes the CSP entered by the user on the console and processes it
func getCredentialsMeta(csp string) ([]string, error) {
	// Look up the credential input format for the csp.
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

	resp, err := newRequest().SetBody("{}").Post(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	if isVerbose {
		fmt.Println(string(resp.Body()))
	}

	// Extract the "Credential" value out of the JSON result
	var result map[string]interface{}
	err = json.Unmarshal(resp.Body(), &result)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	if credential, ok := result["Credential"].([]interface{}); ok {
		fmt.Printf("Successfully retrieved credential meta information for %s\n", csp)

		// Convert the Credential value into a string array
		credentialArray := make([]string, len(credential))
		for i, v := range credential {
			credentialArray[i] = fmt.Sprintf("%v", v)
		}

		return credentialArray, nil
	} else {
		return nil, fmt.Errorf("Credential key not found in response")
	}

}

// Encrypts the credentials of a CSP.
func processCspCredentialEncrypt() {
	//fmt.Println("Processing CSP Credential Encryption : ", csp)
	selectedCsp := ""

	// When it was passed as a CLI option
	if csp != "" {
		selectedCsp = strings.ToLower(csp)
	} else {
		var err error
		// Let the user pick a CSP on the console
		selectedCsp, err = selectCspFromCLI()
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
	}

	// Store selectedCsp lowercased
	//selectedCsp = strings.ToLower(selectedCsp)

	// Print a message saying the credentials of the selected CSP are being processed
	fmt.Printf("\nProcessing authentication information for selected [%s] CSP\n", selectedCsp)

	// Get the credential meta information for the CSP
	credentialMeta, err := getCredentialsMeta(selectedCsp)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// In verbose mode, print the credential keys that have to be entered
	if isVerbose {
		// fmt.Println("Credential Meta :", credentialMeta)
		fmt.Println("The following credential information is required:")
		for _, key := range credentialMeta {
			fmt.Println(key)
		}
		fmt.Println()
	}

	// Read the credential values from the user
	credentials, err := inputCredentialsFromCli(credentialMeta)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Get the PublicKey and the PublicKeyTokenId
	publicKeyResponse, err := getPublicKey()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if isVerbose {
		// Print the PublicKey and PublicKeyTokenId values
		fmt.Println("PublicKeyTokenId:", publicKeyResponse.PublicKeyTokenId)
		fmt.Println("PublicKey:", publicKeyResponse.PublicKey)
	}

	// Encrypt the credentials with the PublicKey
	encryptedCredentials, encryptedAesKey, err := encryptCredentialsWithPublicKey(publicKeyResponse.PublicKey, credentials)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	if isVerbose {
		fmt.Println("Encrypted Credentials:", encryptedCredentials)
		fmt.Println("Encrypted AES Key:", encryptedAesKey)
	}

	// Send the encrypted credentials to the server
	payload := map[string]interface{}{
		"credentialHolder":                 "admin",
		"providerName":                     selectedCsp,
		"publicKeyTokenId":                 publicKeyResponse.PublicKeyTokenId,
		"encryptedClientAesKeyByPublicKey": encryptedAesKey,
		"credentialKeyValueList":           encryptedCredentials,
	}

	// Pretty-print the payload
	if isVerbose {
		fmt.Println("=============================================")
		payloadJSON, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			fmt.Println("Error marshalling payload:", err)
		} else {
			fmt.Println("Payload:")
			fmt.Println(string(payloadJSON))
		}
		fmt.Println("=============================================")
	}

	result, err := sendCredentials(payload)
	if err != nil {
		fmt.Println("Error:", err)
	} else {
		fmt.Println("Result:", result)
	}
}

// Reads the CSP credentials from the user on the console and returns them as a map
func inputCredentialsFromCli(credentialMeta []string) (map[string]string, error) {
	credentials := make(map[string]string)
	reader := bufio.NewReader(os.Stdin)

	// Save the terminal state
	oldState, err := term.GetState(int(syscall.Stdin))
	if err != nil {
		return nil, fmt.Errorf("Error getting terminal state: %v", err)
	}
	// Make sure the terminal is not left with echo turned off if we bail out on an input error
	defer func() { _ = term.Restore(int(syscall.Stdin), oldState) }()

	for {
		// ================================
		// Read the CSP credentials
		// ================================
		for _, key := range credentialMeta {
			fmt.Printf("Please enter %s: ", key)
			// value, err := reader.ReadString('\n')
			// Hide the input
			bytePassword, err := term.ReadPassword(int(syscall.Stdin))
			if err != nil {
				return nil, fmt.Errorf("Error reading input: %v", err)
			}
			value := string(bytePassword)
			fmt.Println() // line break
			credentials[key] = strings.TrimSpace(value)
		}

		// Restore the terminal settings
		if err := term.Restore(int(syscall.Stdin), oldState); err != nil {
			return nil, fmt.Errorf("Error restoring terminal state: %v", err)
		}

		// ================================
		// Ask whether the entered values should be reviewed
		// ================================
		for {
			fmt.Print("Do you want to review the entered credentials? (yes/no): ")
			review, err := reader.ReadString('\n')
			if err != nil {
				return nil, fmt.Errorf("Error reading input: %v", err)
			}
			review = strings.TrimSpace(strings.ToLower(review))

			if review == "yes" {
				// Print the entered values so they can be reviewed
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
		// Ask whether the entered values should be reviewed
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

// PublicKeyResponse struct definition
type PublicKeyResponse struct {
	PublicKeyTokenId string `json:"publicKeyTokenId"`
	PublicKey        string `json:"publicKey"`
}

// Retrieves the PublicKey and the PublicKeyTokenId from the server.
func getPublicKey() (*PublicKeyResponse, error) {
	fmt.Println("Retrieving public key and public key token id")
	url := serviceInfo.BaseURL + GET_PUBLICKEY_URL
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	resp, err := newRequest().Get(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	if isVerbose {
		fmt.Println(string(resp.Body()))
	}

	// Convert the JSON result into a PublicKeyResponse struct
	var publicKeyResponse PublicKeyResponse
	err = json.Unmarshal(resp.Body(), &publicKeyResponse)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	return &publicKeyResponse, nil
}

// Function that adds PKCS7 padding
// The only caller passes aes.BlockSize() (16), so padding lands in 1..16 and
// always fits a byte — which PKCS#7 requires anyway, being undefined for block
// sizes of 256 or more.
func pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padText := bytes.Repeat([]byte{byte(padding)}, padding) // #nosec G115 -- padding is 1..blockSize (16 here), always within byte range
	return append(data, padText...)
}

// Function that removes PKCS7 padding
func pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("invalid padding size")
	}
	padding := int(data[length-1])
	if padding > length {
		return nil, fmt.Errorf("invalid padding size")
	}
	return data[:length-padding], nil
}

func encryptCredentialsWithPublicKey(publicKeyPem string, credentials map[string]string) ([]map[string]string, string, error) {
	block, _ := pem.Decode([]byte(publicKeyPem))
	if block == nil {
		return nil, "", fmt.Errorf("Failed to decode PEM block containing public key")
	}

	rsaPublicKey, err := x509.ParsePKCS1PublicKey(block.Bytes)
	if err != nil {
		return nil, "", fmt.Errorf("failed to parse public key: %v", err)
	}

	// Generate AES key
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
		return nil, "", fmt.Errorf("failed to generate AES key: %v", err)
	}

	// Encrypt credentials using AES (CBC mode, PKCS7 padding)
	// 	encryptedCredentials := []map[string]interface{}{}
	encryptedCredentials := []map[string]string{}
	for k, v := range credentials {
		aesCipher, err := aes.NewCipher(aesKey)
		if err != nil {
			return nil, "", fmt.Errorf("failed to create AES cipher: %v", err)
		}

		iv := make([]byte, aesCipher.BlockSize())
		if _, err := io.ReadFull(rand.Reader, iv); err != nil {
			return nil, "", fmt.Errorf("failed to generate IV: %v", err)
		}

		cbc := cipher.NewCBCEncrypter(aesCipher, iv)
		paddedValue := pkcs7Pad([]byte(v), aesCipher.BlockSize())
		ciphertext := make([]byte, len(paddedValue))
		cbc.CryptBlocks(ciphertext, paddedValue)

		encryptedCredentials = append(encryptedCredentials, map[string]string{
			"key":   k,
			"value": base64.StdEncoding.EncodeToString(append(iv, ciphertext...)),
		})
	}

	// Encrypt AES key using RSA public key with OAEP padding and SHA-256
	encryptedAesKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPublicKey, aesKey, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encrypt AES key: %v", err)
	}

	return encryptedCredentials, base64.StdEncoding.EncodeToString(encryptedAesKey), nil
}

// Sends the encrypted credentials to the server
func sendCredentials(payload map[string]interface{}) (map[string]interface{}, error) {
	fmt.Println("Sending encrypted credentials to server")

	// Like every other call, this uses the address resolved from --host/--port/--user/--password.
	// That is what makes it possible to initialize the Tumblebug of several servers from a single machine.
	// (The default is still http://localhost:1323/tumblebug)
	url := serviceInfo.BaseURL + POST_CREDENTIAL_URL
	if isVerbose {
		fmt.Println("Request Url : ", url)
	}

	// Convert the payload to JSON
	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("Error marshalling payload: %v", err)
	}

	resp, err := newRequest().
		SetHeader("Content-Type", "application/json").
		SetBody(reqBody).
		Post(url)
	if err != nil {
		fmt.Println("Error:", err)
		return nil, fmt.Errorf("Error: %v", err)
	}

	// Check the response
	body := resp.Body() // []byte
	if isVerbose {
		fmt.Println(string(body))
	}

	// Anything but a 2xx means the registration failed, so return an error along with the response body
	if resp.StatusCode() < 200 || resp.StatusCode() > 299 {
		return nil, fmt.Errorf("credential registration failed with status %d: %s", resp.StatusCode(), strings.TrimSpace(string(body)))
	}

	// Parse the response and return it in the expected return type
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		return nil, fmt.Errorf("Error parsing JSON: %v", err)
	}

	// Print a message saying the CSP credential registration is complete
	fmt.Println("CSP Credential registration completed successfully")

	return result, nil
}

var credentialCmd = &cobra.Command{
	Use:   "credential",
	Short: "Registration of CSP-Specific Credentials and Default Resources",
	Long: `Supports the registration of CSP credentials and initial data
	The basic information of the subsystem is utilized from the api.yaml file, but the user can change the API authentication information including host and port.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		isInit = false

		// Do not print the help when a subcommand was given.
		if len(args) == 0 && cmd.Flags().NFlag() == 0 && cmd.HasSubCommands() {
			//fmt.Println(cmd.Help())
			_ = cmd.Help()
			return
		}

		// Process the configuration file
		viper.SetConfigFile(configFile)
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Printf("Error reading config file: %s\n", err)
			return
		}

		// Process the information of the service to call
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

		// publicKeyResponse, err := getPublicKey()
		// if err != nil {
		// 	fmt.Println("Error:", err)
		// 	return
		// }

		// if isVerbose {
		// 	// Print the PublicKey and PublicKeyTokenId values
		// 	fmt.Println("PublicKeyTokenId:", publicKeyResponse.PublicKeyTokenId)
		// 	fmt.Println("PublicKey:", publicKeyResponse.PublicKey)
		// }

		// Encrypt the credentials of the CSP
		processCspCredentialEncrypt()
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
