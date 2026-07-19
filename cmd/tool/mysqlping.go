// docker compose의 컨테이너 실행 순서 보장을 위해 MySQL의 Ready 상태를 확인하는 기능을 수행
package tool

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"database/sql"
	"log"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

var user string
var password string
var host string
var port string
var database string
var isVerbose bool

var mysqlpingCmd = &cobra.Command{
	Use:   "mysqlping",
	Short: "Function to check the Ready status of MySQL",
	Long: `Function to check the Health status of the MySQL container in Docker Compose
This command checks the readiness of a MySQL database by attempting to connect to it using the provided connection information.
The connection information is obtained from environment variables, and values provided via flags take precedence over the environment variables.
database name is optional, and if not provided, the connection will be made to the MySQL server without specifying a database.

Environment Variables:
  MYSQL_USER       MySQL username
  MYSQL_PASSWORD   MySQL password
  MYSQL_HOST       MySQL host
  MYSQL_PORT       MySQL port
  MYSQL_DATABASE   MySQL database name

Flags:
  --user       MySQL username (default: "root")
  --password   MySQL password
  --host       MySQL host (default: "localhost")
  --port       MySQL port (default: "3306")
  --database   MySQL database name
  	`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		//fmt.Println("len(args) ", len(args))
		//fmt.Println("cmd.Flags().NFlag() ", cmd.Flags().NFlag())

		// 아규먼트나 플래그가 입력 되지 않은 경우에만 도움말 출력 후 종료
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			// 환경 변수가 모두 설정되었으면 도움말 출력 없이 종료
			if checkConfig(cmd) {
				return
			}
			_ = cmd.Help()
			os.Exit(1) // Docker Compose의 Health Check 사용을 감안해서 에러로 리턴 함.
		} else {
			// verbose 플래그만 설정된 경우에는 도움말 출력
			if len(args) == 0 && cmd.Flags().NFlag() == 1 && isVerbose {
				_ = cmd.Help()
				fmt.Print("\n\n")
			}

			if !checkConfig(cmd) {
				log.Println("Please set the MySQL connection information using flags or environment variables.")
				os.Exit(1) // 비정상 종료
			}
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		if dbPing() {
			os.Exit(0) // 정상 종료
		} else {
			os.Exit(1) // 비정상 종료
		}
	},
}

func dbPing() bool {
	// DSN 구성
	log.Printf("Checking MySQL[%s:%s] connection...\n", host, port)

	var dsn string
	if database == "" {
		dsn = user + ":" + password + "@tcp(" + host + ":" + port + ")"
	} else {
		dsn = user + ":" + password + "@tcp(" + host + ":" + port + ")/" + database
	}

	if isVerbose {
		log.Println("DSN:", maskDSN(dsn))
	}

	// MySQL 연결 테스트
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Println("MySQL connection failed:", err)
		os.Exit(1) // 비정상 종료
	}
	defer db.Close()

	// MySQL Ping 테스트
	if err := db.Ping(); err != nil {
		log.Println("MySQL ping failed:", err)
		os.Exit(1) // 비정상 종료
	}

	log.Println("MySQL is healthy.")
	return true
}

// maskDSN replaces the password in a MySQL DSN with ***, so a DSN can be logged
// without printing the credential.
//
// This matters more here than it looks: mysqlping runs as a docker compose
// healthcheck, so it is executed every few seconds for the lifetime of the
// stack. With -v enabled, the DSN was written to the container log on every one
// of those runs — a plaintext DB password accumulating in a log anyone with
// access to the daemon can read.
//
// The DSN format is user:password@tcp(host:port)[/database]. The password is
// whatever sits between the first ':' and the last '@' before the address, so
// the split is on the last '@' — a password may legitimately contain '@'.
func maskDSN(dsn string) string {
	at := strings.LastIndex(dsn, "@")
	if at < 0 {
		return dsn
	}
	credentials, rest := dsn[:at], dsn[at:]
	colon := strings.Index(credentials, ":")
	if colon < 0 {
		return dsn
	}
	return credentials[:colon] + ":***" + rest
}

// maskSecret reports a secret's presence without revealing it.
//
// Deliberately stricter than common.MaskSecret, which keeps a two-character
// prefix so a user can tell one credential from another. That trade-off is
// right for an interactive `-v` run and wrong here: mysqlping is a docker
// compose healthcheck, so with -v enabled it re-prints this line every few
// seconds for the lifetime of the stack. A prefix repeated thousands of times
// into a container log is worth more to someone reading the log than it is to
// the operator, who ran the command once and knows what they typed.
func maskSecret(s string) string {
	if s == "" {
		return "(empty)"
	}
	return "***"
}

// DB 접속을 위한 환경 변수 값을 체크 함.
//
// Precedence: a flag the user actually typed always wins over the matching
// environment variable, which is what the command's help text promises.
//
// The previous version could not honour that for --host and --port, because it
// decided "the user did not pass a flag" by comparing the value against the
// default. `--host localhost` is indistinguishable from an absent --host under
// that test, so MYSQL_HOST won and the connection went somewhere the user did
// not ask for — the opposite of the documented behaviour, and silent.
// cmd.Flags().Changed answers the question directly.
func checkConfig(cmd *cobra.Command) bool {
	if isVerbose {
		log.Println("Checking MySQL connection information...")
	}

	// 플래그로 명시하지 않은 값만 환경 변수로부터 읽어오기
	if !cmd.Flags().Changed("user") && os.Getenv("MYSQL_USER") != "" {
		user = os.Getenv("MYSQL_USER")
	}

	if !cmd.Flags().Changed("password") && os.Getenv("MYSQL_PASSWORD") != "" {
		password = os.Getenv("MYSQL_PASSWORD")
	}

	if !cmd.Flags().Changed("host") && os.Getenv("MYSQL_HOST") != "" {
		host = os.Getenv("MYSQL_HOST")
	}

	if !cmd.Flags().Changed("port") && os.Getenv("MYSQL_PORT") != "" {
		port = os.Getenv("MYSQL_PORT")
	}

	if !cmd.Flags().Changed("database") && os.Getenv("MYSQL_DATABASE") != "" {
		database = os.Getenv("MYSQL_DATABASE")
	}

	if isVerbose {
		log.Println("user:", user)
		log.Println("password:", maskSecret(password))
		log.Println("host:", host)
		log.Println("port:", port)
		log.Println("database:", database)
	}

	if user == "" || password == "" {
		return false
	}
	return true

}

func init() {
	toolCmd.AddCommand(mysqlpingCmd)

	// Add flags for MySQL connection
	mysqlpingCmd.Flags().StringVarP(&user, "user", "u", "", "Username for MySQL connection")
	mysqlpingCmd.Flags().StringVarP(&password, "password", "p", "", "Password for MySQL connection")
	mysqlpingCmd.Flags().StringVarP(&host, "host", "", "localhost", "The server address where MySQL is running (Default: localhost)")
	mysqlpingCmd.Flags().StringVarP(&port, "port", "", "3306", "The port number MySQL is using (Default: 3306)")
	mysqlpingCmd.Flags().StringVarP(&database, "database", "d", "", "The database name to connect to (Default: none)")

	mysqlpingCmd.Flags().BoolVarP(&isVerbose, "verbose", "v", false, "Show more detail information for debugging")
}
