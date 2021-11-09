// Command login_2fa performs two-factor authentication for
// a C by GE (Cync) account, returning a session as JSON
// if the login succeeds.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/unixpickle/cbyge"
	"github.com/unixpickle/essentials"
)

func main() {
	var email string
	var password string
	flag.StringVar(&email, "email", "", "user email")
	flag.StringVar(&password, "password", "", "user password")
	flag.Parse()

	if email == "" || password == "" {
		essentials.Die("Must provide -email and -password flags. See -help.")
	}

	err := cbyge.Login2FAStage1(email, "")
	essentials.Must(err)

	fmt.Print("Enter verification code: ")

	r := bufio.NewReader(os.Stdin)
	code, err := r.ReadString('\n')
	essentials.Must(err)
	info, err := cbyge.Login2FAStage2(email, password, "", strings.TrimSpace(code))
	essentials.Must(err)

	data, _ := json.Marshal(info)
	fmt.Println(string(data))
}
