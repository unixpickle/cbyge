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
	flag.StringVar(&password, "pass", "", "user password")
	flag.Parse()

	cb, err := cbyge.Login2FA(email, password, "")
	essentials.Must(err)

	r := bufio.NewReader(os.Stdin)
	code, err := r.ReadString('\n')
	essentials.Must(err)
	info, err := cb(strings.TrimSpace(code))
	essentials.Must(err)

	data, _ := json.Marshal(info)
	fmt.Println(string(data))
}
