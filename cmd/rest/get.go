package rest

import (
	"github.com/spf13/cobra"
)

/*
var headers []string
var username string
var password string
var showHeaders bool
*/

// restGetCmd represents the restGet command
var restGetCmd = &cobra.Command{
	Use:   "get",
	Short: "REST API calls with GET methods",
	Long: `REST API calls with GET methods. For example:

	rest get -u default -p default http://localhost:1323/tumblebug/health
	rest get https://reqres.in/api/users/2
	rest get https://reqres.in/api/users?page=2
	rest get https://reqres.in/api/users?delay=3
`,
	//Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 { // print the help message when no argument is given
			//fmt.Println(cmd.Help())
			_ = cmd.Help()
			return
		}

		url := args[0]
		runRequest(url, req.Get)
	},
}

func init() {
	restCmd.AddCommand(restGetCmd)
}
