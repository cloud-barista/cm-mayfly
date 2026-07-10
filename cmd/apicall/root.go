/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package apicall

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/cmd"
	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configFile string

var serviceName string
var actionName string

// var method string
var isInit bool
var isListMode bool
var isVerbose bool
var pathParam string
var queryString string

var client = common.NewHTTPClient()
var req = client.R()
var sendData string
var inputFileData string
var outputFile string

// auth : About changing credentials from the CLI
var username string
var password string
var authToken string

/*
type ServiceInfo struct {
	BaseURL string `yaml:"baseurl"`
	Auth    struct {
		Type     string `yaml:"type"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
	} `yaml:"auth"`
	ResourcePath string `yaml:"resourcePath"`
	Method       string `yaml:"method"`
}
*/

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

// apiCmd represents the svc command
var apiCmd = &cobra.Command{
	Use:   "api",
	Short: "Call the Cloud-Migrator system's Open APIs as services and actions",
	Long: `Call the action of the service defined in api.yaml. For example:
./mayfly api --help
./mayfly api --list
./mayfly api --service cb-spider --list
./mayfly api --service cb-spider --action ListCloudOS
./mayfly api --service cb-spider --action GetCloudDriver --pathParam driver_name:AWS
./mayfly api --service cb-spider --action GetRegionZone --pathParam region_name:ap-northeast-3 --queryString ConnectionName=aws-config01
./mayfly api --service cb-tumblebug --action Getmcivm --pathParam "nsId:ns01 mciId:mci01 vmId:vm01" --queryString "option=status&accessInfo=showSshKey"
./mayfly api --service cm-beetle --action Deleteinfra --pathParam "nsId:mig01 mciId:mmci01" --queryString "option=terminate"
`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		//fmt.Printf("len(args) : %d\n", len(args))
		//fmt.Printf("cmd.Flags().NFlag() : %d\n", cmd.Flags().NFlag())
		//fmt.Printf("cmd.HasSubCommands() : %v\n", !cmd.HasSubCommands())

		isInit = false
		// Do not print help when a tool subcommand is entered.
		if len(args) == 0 && cmd.Flags().NFlag() == 0 && cmd.HasSubCommands() {
			//fmt.Println(cmd.Help())
			cmd.Help()
			return
		}

		//fmt.Println("============ arguments :  " + strconv.Itoa(len(args)))
		//fmt.Println("============ flag count :  " + strconv.Itoa(cmd.Flags().NFlag()))

		//viper.AddConfigPath("../conf")
		viper.SetConfigFile(configFile)

		// Read the config file
		err := viper.ReadInConfig()
		if err != nil {
			fmt.Printf("Error reading config file: %s\n", err)
			return
		}
		isInit = true

		//fmt.Println("cliSpecVersion : ", viper.GetString("cliSpecVersion"))
		//fmt.Println("Loaded configurations:", viper.AllSettings())
		if isVerbose {
			client.SetDebug(true)
			//spew.Dump(viper.AllSettings())
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		if !isInit {
			return
		}

		//
		// handle the list command
		//
		if isListMode {
			if isVerbose {
				fmt.Println("List Mode")
			}
			if serviceName == "" {
				showServiceList()
			} else if actionName == "" {
				showActionList(serviceName)
			} else {
				fmt.Printf("Both the service and action were specified.\nThe list no longer exists to lookup.\n")
			}

			return
		}

		// process the service info to call
		errParse := parseRequestInfo()
		if errParse != nil {
			fmt.Println(errParse)
			return
		}

		if isVerbose {
			//spew.Dump(serviceInfo)

			fmt.Println("")
			fmt.Println("Base URL:", serviceInfo.BaseURL)
			fmt.Println("Auth Type:", serviceInfo.Auth.Type)
			fmt.Println("Username:", serviceInfo.Auth.Username)
			fmt.Println("Password:", serviceInfo.Auth.Password)
			fmt.Println("Token:", serviceInfo.Auth.Token)
			fmt.Println("ResourcePath:", serviceInfo.ResourcePath)
			fmt.Println("Method:", serviceInfo.Method)
		}

		fmt.Println("\nservice calling...")
		errRest := callRest()
		if errRest != nil {
			fmt.Println(errRest)
			return
		}
	},
}

// query the service list
func showServiceList() {
	services := viper.GetStringMap("services")

	fmt.Printf("============\n")
	fmt.Printf("Service list\n")
	fmt.Printf("============\n")

	for serviceName := range services {
		fmt.Println(serviceName)
	}
}

// query the action list under a service
func showActionList(serviceName string) {
	spiderActions := viper.GetStringMap("serviceActions." + serviceName)

	fmt.Printf("==============================\n")
	fmt.Printf("[%s] Service Actions list\n", serviceName)
	fmt.Printf("==============================\n")
	for actionName := range spiderActions {
		fmt.Println(actionName)
	}
}

// Organize the service info to call based on the input values.
func parseRequestInfo() error {
	// validate the service
	if serviceName == "" {
		return errors.New("no service is specified to call")
	}

	if !viper.IsSet("services." + serviceName) {
		//return errors.New("information about the service [" + serviceName + "] you are trying to call does not exist")
		return errors.New("the name of the service[" + serviceName + "] you want to call is not on the list of supported services.\nPlease check the api.yaml configuration file or the list of available services")
	}

	// validate the action
	if actionName == "" {
		return errors.New("no action name is specified to call")
	}

	if !viper.IsSet("serviceActions." + serviceName + "." + actionName) {
		return errors.New("the requested action[" + actionName + "] does not exist for the service[" + serviceName + "] you are trying to call\nPlease check the api.yaml configuration file or the list of available actions for the service you want to call.")
	}

	// parse the service info
	err := viper.UnmarshalKey("services."+serviceName, &serviceInfo)
	if err != nil {
		return err
	}

	// When an api.yaml auth value is written as ${VAR}, resolve it from the
	// environment. Priority: (CLI flag) > process OS env > conf/docker/.env file.
	// CLI flags override further below, so env resolution is done first here.
	// There is no silent default fallback — if no source holds the value it
	// stays empty and a warning is printed.
	if v, unset := common.ResolveEnvRef(serviceInfo.Auth.Username); unset {
		fmt.Fprintf(os.Stderr, "Warning: api.yaml auth username for service %q references an unset env var (%s); sending empty credential.\n", serviceName, serviceInfo.Auth.Username)
		serviceInfo.Auth.Username = ""
	} else {
		serviceInfo.Auth.Username = v
	}
	if v, unset := common.ResolveEnvRef(serviceInfo.Auth.Password); unset {
		fmt.Fprintf(os.Stderr, "Warning: api.yaml auth password for service %q references an unset env var (%s); sending empty credential.\n", serviceName, serviceInfo.Auth.Password)
		serviceInfo.Auth.Password = ""
	} else {
		serviceInfo.Auth.Password = v
	}

	// handle the case where auth info is passed via the CLI
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

	// parse the action info
	err = viper.UnmarshalKey("serviceActions."+serviceName+"."+actionName, &serviceInfo)
	if err != nil {
		return err
	}

	if serviceInfo.ResourcePath == "" {
		return errors.New("couldn't find the ResourcePath information for the action to call\nPlease check the api.yaml configuration file")
	}

	// handle the variable URI
	errParam := parsePathParam()
	if errParam != nil {
		//fmt.Println(errParam)
		return errParam
	}

	// handle the query string
	if queryString != "" {
		// check whether the queryString value starts with ?
		startsWithQuestionMark := strings.HasPrefix(queryString, "?")

		// when ResourcePath ends with ?
		if strings.HasSuffix(serviceInfo.ResourcePath, "?") {
			if startsWithQuestionMark {
				serviceInfo.ResourcePath = serviceInfo.ResourcePath + queryString[1:]
			} else {
				serviceInfo.ResourcePath = serviceInfo.ResourcePath + queryString
			}
		} else {
			if startsWithQuestionMark {
				serviceInfo.ResourcePath = serviceInfo.ResourcePath + queryString
			} else {
				serviceInfo.ResourcePath = serviceInfo.ResourcePath + "?" + queryString
			}
		}
	}

	return nil
}

// Handle the variable path.
func parsePathParam() error {
	if isVerbose {
		fmt.Println("pathParam:", pathParam)
		fmt.Println("ResourcePath:", serviceInfo.ResourcePath)
		fmt.Println("checking path paramter infomation...")
	}

	//handle Path parameters
	if strings.Contains(serviceInfo.ResourcePath, "{") {
		if pathParam == "" {
			return errors.New("couldn't find uri path parameter(key:value) information for URI PATH\nThis URI requires the following path parameter information\n" + serviceInfo.ResourcePath)
		}

		//handle the variable path
		pathParams := make(map[string]string)
		params := strings.Fields(pathParam)
		for _, param := range params {
			keyValue := strings.Split(param, ":")
			if len(keyValue) == 2 {
				//key := strings.ToLower(keyValue[0])
				key := keyValue[0]
				value := keyValue[1]
				pathParams[key] = value
			}
		}

		// replace the resourcePath keys case-sensitively
		for key, value := range pathParams {
			//lowerKey := strings.ToLower(key)
			placeholder := "{" + key + "}"
			//serviceInfo.ResourcePath = strings.Replace(serviceInfo.ResourcePath, placeholder, value, -1)
			serviceInfo.ResourcePath = strings.Replace(serviceInfo.ResourcePath, placeholder, value, -1)
		}

		if strings.Contains(serviceInfo.ResourcePath, "{") {
			return errors.New("couldn't find all uri path parameter(key:value) information for URI PATH\nThis URI requires the following addtional path parameter information\nkey names used for URI mapping are case sensitive.\n" + serviceInfo.ResourcePath)
		}
	}

	if isVerbose {
		fmt.Println("ResourcePath:", serviceInfo.ResourcePath)
	}
	return nil
}

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

// handle auth
func SetAuth() {
	switch strings.ToLower(serviceInfo.Auth.Type) {
	case "none", "":
		// do nothing when no auth is required
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

// func SetReqData(req *resty.Request) {
func SetReqData() error {
	if inputFileData != "" {
		if isVerbose {
			fmt.Printf("use [%s] data file\n", inputFileData)
		}

		// read the data from the file
		data, err := ioutil.ReadFile(inputFileData)
		if err != nil {
			return err
		}
		req.SetBody(data)
	} else {
		if isVerbose {
			fmt.Printf("request data : %s\n", sendData)
		}
		req.SetBody(sendData)
	}

	return nil
}

func ProcessResultInfo(resp *resty.Response) {
	if isVerbose {
		fmt.Println("  Headers:")
		for key, values := range resp.Header() {
			for _, value := range values {
				fmt.Printf("%s: %s\n", key, value)
			}
		}
		fmt.Println("")
	}

	fmt.Println(string(resp.Body()))
}

// Call the REST API.
func callRest() error {
	var resp *resty.Response
	var err error

	SetAuth()          // handle auth
	err = SetReqData() // handle the data to send
	if err != nil {
		return err
	}

	//specify the output file
	if outputFile != "" {
		req.SetOutput(outputFile)
	}

	url := serviceInfo.BaseURL + serviceInfo.ResourcePath

	switch strings.ToLower(serviceInfo.Method) {
	case "get":
		resp, err = req.Get(url)
	case "post":
		resp, err = req.Post(url)
	case "put":
		resp, err = req.Put(url)
	case "delete":
		resp, err = req.Delete(url)
	case "patch":
		resp, err = req.Patch(url)
	}

	if err != nil {
		return err
	}
	ProcessResultInfo(resp)

	return nil
}

func init() {
	//apiCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "./conf/api.yaml", "config file")
	apiCmd.PersistentFlags().StringVarP(&configFile, "config", "c", common.API_FILE, "config file")

	// Add flags for basic authentication
	apiCmd.PersistentFlags().StringVarP(&username, "authUser", "", "", "Username for basic authentication") // - sets the basic authentication header in the HTTP request
	apiCmd.PersistentFlags().StringVarP(&password, "authPassword", "", "", "Password for basic authentication")

	// set the auth token
	apiCmd.PersistentFlags().StringVarP(&authToken, "authToken", "", "", "sets the auth token of the 'Authorization' header for all HTTP requests.(The default auth scheme is 'Bearer')")
	//apiCmd.PersistentFlags().StringVarP(&authScheme, "authScheme", "", "", "sets the auth scheme type in the HTTP request.(Exam. OAuth)(The default auth scheme is Bearer)")

	apiCmd.PersistentFlags().StringVarP(&serviceName, "service", "s", "", "Service to perform")
	apiCmd.PersistentFlags().StringVarP(&actionName, "action", "a", "", "Action to perform")
	//apiCmd.PersistentFlags().StringVarP(&method, "method", "m", "", "HTTP Method")
	apiCmd.PersistentFlags().BoolVarP(&isVerbose, "verbose", "v", false, "Show more detail information")
	apiCmd.PersistentFlags().StringVarP(&pathParam, "pathParam", "p", "", "Variable path info set \"key1:value1 key2:value2\" for URIs (separated by space)")
	apiCmd.PersistentFlags().StringVarP(&queryString, "queryString", "q", "", "Query string to add to URIs (format: \"param1=value1\" or \"param1=value1&param2=value2\")")

	apiCmd.Flags().BoolVarP(&isListMode, "list", "l", false, "Show Service or Action list")
	apiCmd.PersistentFlags().StringVarP(&sendData, "data", "d", "", "Data to send to the server")
	apiCmd.PersistentFlags().StringVarP(&inputFileData, "file", "f", "", "Data to send to the server from file")
	apiCmd.PersistentFlags().StringVarP(&outputFile, "output", "o", "", "<file> Write to file instead of stdout")

	cmd.RootCmd.AddCommand(apiCmd)
}
