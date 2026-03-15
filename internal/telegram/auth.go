package telegram

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
	"golang.org/x/term"
)

type TerminalAuth struct {
	phone string
}

func NewTerminalAuth(phone string) *TerminalAuth {
	return &TerminalAuth{phone: phone}
}

var _ auth.UserAuthenticator = (*TerminalAuth)(nil)

func (a *TerminalAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a *TerminalAuth) Password(_ context.Context) (string, error) {
	fmt.Print("Enter 2FA password: ")
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(password), nil
}

func (a *TerminalAuth) Code(_ context.Context, _ *tg.AuthSentCode) (string, error) {
	fmt.Print("Enter auth code: ")
	reader := bufio.NewReader(os.Stdin)
	code, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(code), nil
}

func (a *TerminalAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (a *TerminalAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
}
