// Command loopnet-login-probe validates the pasted browser cookie by
// fetching /myloopnet/ and printing the dashboard title.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	loopnet "github.com/teslashibe/loopnet-go"
)

func main() {
	auth := loopnet.Auth{
		CookieHeader: os.Getenv("LOOPNET_COOKIE_HEADER"),
		UserAgent:    os.Getenv("LOOPNET_USER_AGENT"),
	}
	c, err := loopnet.New(auth)
	if err != nil {
		fmt.Fprintln(os.Stderr, "init:", err)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Required:")
		fmt.Fprintln(os.Stderr, "  LOOPNET_COOKIE_HEADER='<paste of entire cookie: header from any DevTools→Network")
		fmt.Fprintln(os.Stderr, "                          request on www.loopnet.com after you have logged in>'")
		fmt.Fprintln(os.Stderr, "  LOOPNET_USER_AGENT='<exact UA string from same browser tab>'  (recommended)")
		os.Exit(1)
	}
	user, err := c.Login(context.Background())
	if err != nil {
		if errors.Is(err, loopnet.ErrBotChallenge) {
			fmt.Fprintln(os.Stderr, "Akamai bot challenge — refresh _abck/bm_sz/ak_bmsc cookies from a real browser session.")
		}
		fmt.Fprintln(os.Stderr, "login:", err)
		os.Exit(1)
	}
	fmt.Printf("logged in (logged_in=%v) title=%q\n", user.LoggedIn, user.DisplayName)
}
