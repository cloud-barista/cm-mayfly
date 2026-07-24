/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package rest

import (
	"github.com/spf13/cobra"
)

// restPutCmd represents the restPut command
var restPutCmd = &cobra.Command{
	Use:   "put",
	Short: "REST API calls with PUT methods",
	Long: `REST API calls with PUT methods. For example:

	rest put https://reqres.in/api/users/2 -d '
	{
		"name": "morpheus",
		"job": "leader"
	}'`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 { // print the help message when no argument is given
			//fmt.Println(cmd.Help())
			_ = cmd.Help()
			return
		}

		url := args[0]
		runRequest(url, req.Put)
	},
}

func init() {
	restCmd.AddCommand(restPutCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// restPutCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// restPutCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
