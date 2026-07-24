/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package rest

import (
	"github.com/spf13/cobra"
)

// restPostCmd represents the restPost command
var restPostCmd = &cobra.Command{
	Use:   "post",
	Short: "REST API calls with POST methods",
	Long: `REST API calls with POST methods For example:

	rest post https://reqres.in/api/users -d '
	{
		"name": "morpheus",
		"job": "leader"
	}'`,

	//Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		//fmt.Println("rest post called")
		if len(args) < 1 { // print the help message when no argument is given
			//fmt.Println(cmd.Help())
			_ = cmd.Help()
			return
		}

		url := args[0]
		runRequest(url, req.Post)
	},
}

func init() {
	restCmd.AddCommand(restPostCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// restPostCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// restPostCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
