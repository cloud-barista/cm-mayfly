package docker

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/cm-mayfly/cm-mayfly/common"
	"github.com/spf13/cobra"
)

// removeCmd represents the remove command
var removeCmd = &cobra.Command{
	Use:   "remove",
	Short: "Stop and Remove Cloud-Migrator System",
	Long:  `Stop and Remove Cloud-Migrator System. Stop and Remove Cloud-Migrator runtimes and related container images and meta-DB if necessary`,
	Run: func(cmd *cobra.Command, args []string) {

		fmt.Println("\n[Remove Cloud-Migrator]")
		fmt.Println()

		var cmdStr string
		var removeOptions string
		if volFlag && imgFlag || allFlag {
			removeOptions = "--volumes --rmi all"
		} else if volFlag {
			removeOptions = "--volumes"
		} else if imgFlag {
			removeOptions = "--rmi all"
		} else {
			removeOptions = ""
		}

		// 삭제 대상 정보 표시
		fmt.Println("Removal Target:")

		// 서비스 대상 표시
		if ServiceName == "" {
			fmt.Println("  Services: All services")
		} else {
			fmt.Printf("  Services: %s\n", ServiceName)
		}

		// 삭제 범위 표시
		if allFlag {
			fmt.Println("  Scope: Containers + Images + Volumes (all)")
		} else if volFlag && imgFlag {
			fmt.Println("  Scope: Containers + Images + Volumes")
		} else if volFlag {
			fmt.Println("  Scope: Containers + Volumes")
		} else if imgFlag {
			fmt.Println("  Scope: Containers + Images")
		} else {
			fmt.Println("  Scope: Containers only (images preserved)")
		}
		fmt.Println()

		// 추가 옵션 안내
		if !imgFlag && !volFlag && !allFlag {
			fmt.Println("Additional Options:")
			fmt.Println("  -i, --images    : Also remove images")
			fmt.Println("  -v, --volumes   : Also remove named volumes (local mounts preserved)")
			fmt.Println("  --all           : Remove everything (images + volumes)")
			fmt.Println()
		} else if volFlag && !allFlag {
			fmt.Println("Note: Named volumes will be removed, but local mount volumes are preserved.")
			fmt.Println()
		}

		// 사용자 확인 요청
		fmt.Print("Do you want to proceed with the removal? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		response, _ := reader.ReadString('\n')
		response = strings.TrimSpace(strings.ToLower(response))

		if response != "y" && response != "yes" {
			fmt.Println("Removal cancelled.")
			return
		}

		cmdStr = fmt.Sprintf("COMPOSE_PROJECT_NAME=%s docker compose -f %s down %s %s", ProjectName, DockerFilePath, removeOptions, ServiceName)

		//fmt.Println(cmdStr)
		common.SysCall(cmdStr)

		SysCallDockerComposePs()
	},
}

var allFlag bool
var volFlag bool
var imgFlag bool

func init() {
	dockerCmd.AddCommand(removeCmd)

	pf := removeCmd.PersistentFlags()
	pf.BoolVarP(&allFlag, "all", "", false, "Remove all images and volumes and networks")
	pf.BoolVarP(&volFlag, "volumes", "v", false, "Remove named volumes declared in the volumes section of the Compose file")
	pf.BoolVarP(&imgFlag, "images", "i", false, "Remove all images")
}
