package apicall

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
)

var swaggerFile string

type Info struct {
	Title   string `json:"title"`
	Version string `json:"version"`
}

type Path struct {
	Description string   `json:"description"`
	Consumes    []string `json:"consumes"`
	Produces    []string `json:"produces"`
	Tags        []string `json:"tags"`
	Summary     string   `json:"summary"`
	// 여기에 더 필요한 필드를 추가할 수 있습니다.
}

type Paths map[string]map[string]Path

type Swagger struct {
	Swagger  string `json:"swagger"`
	Info     Info   `json:"info"`
	BasePath string `json:"basePath"`
	Paths    Paths  `json:"paths"`
	// 여기에 더 필요한 필드를 추가할 수 있습니다.
}

// pullCmd represents the pull command
var toolCmd = &cobra.Command{
	Use:   "tool",
	Short: "Swagger JSON parsing tool to assist in writing api.yaml files",
	Long:  `Swagger JSON parsing tool to assist in writing api.yaml files`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("util")
		parse()
	},
}

func parse() {
	/*
		// JSON 문자열
		data := `{
			"swagger": "2.0",
			"info": {
				"title": "CB-Tumblebug REST API",
				"contact": {
					"name": "API Support",
					"url": "http://cloud-barista.github.io",
					"email": "contact-to-cloud-barista@googlegroups.com"
				},
				"version": "latest"
			},
			"basePath": "/tumblebug",
			"paths": {
				"/cloudInfo": {
					"get": {
						"description": "Get cloud information",
						"consumes": [
							"application/json"
						],
						"produces": [
							"application/json"
						],
						"tags": [
							"[Admin] Multi-Cloud environment configuration"
						],
						"summary": "Get cloud information"
					}
				},
				"/config": {
					"get": {
						"description": "List all configs",
						"consumes": [
							"application/json"
						],
						"produces": [
							"application/json"
						],
						"tags": [
							"[Admin] System environment"
						],
						"summary": "List all configs"
					},
					"post": {
						"description": "Create or Update config (SPIDER_REST_URL, DRAGONFLY_REST_URL, ...)",
						"consumes": [
							"application/json"
						],
						"produces": [
							"application/json"
						],
						"tags": [
							"[Admin] System environment"
						],
						"summary": "Create or Update config",
						"parameters": [
							{
								"description": "Key and Value for configuration",
								"name": "config",
								"in": "body",
								"required": true,
								"schema": {
									"$ref": "#/definitions/common.ConfigReq"
								}
							}
						]
					},
					"delete": {
						"description": "Init all configs",
						"consumes": [
							"application/json"
						],
						"produces": [
							"application/json"
						],
						"tags": [
							"[Admin] System environment"
						],
						"summary": "Init all configs"
					}
				}
			}
		}`
	*/

	// JSON 파일 읽기
	data, err := ioutil.ReadFile(swaggerFile)
	if err != nil {
		fmt.Println("Error reading JSON file:", err)
		return
	}

	// JSON 데이터를 구조체로 언마샬링
	var swagger Swagger
	err = json.Unmarshal(data, &swagger)
	//err := json.Unmarshal([]byte(data), &swagger)
	if err != nil {
		fmt.Println("Error unmarshalling JSON:", err)
		return
	}

	// 기본 정보
	fmt.Println("Swagger Version:", swagger.Swagger)
	fmt.Println("API Title:", swagger.Info.Title)
	fmt.Println("API Version:", swagger.Info.Version)
	fmt.Println("Base Path:", swagger.BasePath)

	// 각 경로에 대한 정보 출력
	for path, methods := range swagger.Paths {
		fmt.Println("Path:", path)
		for method, info := range methods {
			//fmt.Printf("  Method: %s, Description: %s\n", method, info.Description)
			fmt.Printf("  Method: %s, Description: %s\n", method, info.Summary)
		}
	}
}

func init() {
	apiCmd.AddCommand(toolCmd)
	toolCmd.PersistentFlags().StringVarP(&swaggerFile, "file", "f", "../conf/swagger.json", "Swagger JSON file full path")
}
