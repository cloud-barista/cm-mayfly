// Checks whether MySQL is ready, so that docker compose can order container startup accordingly
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

		// Only print the help and exit when neither an argument nor a flag was given
		if len(args) == 0 && cmd.Flags().NFlag() == 0 {
			// If the environment variables are all set, return without printing the help
			if checkConfig(cmd) {
				return
			}
			_ = cmd.Help()
			os.Exit(1) // return an error, since this is used as a Docker Compose health check.
		} else {
			// Print the help when the only flag given is verbose
			if len(args) == 0 && cmd.Flags().NFlag() == 1 && isVerbose {
				_ = cmd.Help()
				fmt.Print("\n\n")
			}

			if !checkConfig(cmd) {
				log.Println("Please set the MySQL connection information using flags or environment variables.")
				os.Exit(1) // failure exit
			}
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		if dbPing() {
			os.Exit(0) // success exit
		} else {
			os.Exit(1) // failure exit
		}
	},
}

func dbPing() bool {
	// Build the DSN
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

	// Test the MySQL connection
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Println("MySQL connection failed:", err)
		os.Exit(1) // failure exit
	}
	defer db.Close()

	// Ping MySQL
	if err := db.Ping(); err != nil {
		log.Println("MySQL ping failed:", err)
		os.Exit(1) // failure exit
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

// Checks the environment variable values used for the DB connection.
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

	// Read from the environment only the values that were not given explicitly as flags
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
