package internal

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

func Launch(command string, args []string) int {

	env, err := buildEnv(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wrapper: %v\n", err)
		return 1
	}

	exitCode, err := runCommand(command, args, env)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wrapper: %v\n", err)
	}

	return exitCode
}

func buildEnv(args []string) ([]string, error) {
	var env = os.Environ()
	var username = searchForUsername(args)
	if username == "" {
		fmt.Fprintf(os.Stderr, "wrapper: Cannot detect username to login\n")
	} else {
		var password, err = retrievePasswordForUser(username)
		if err != nil {
			return env, err
		}
		if password != "" {
			env = append(env, fmt.Sprintf("PGPASSWORD=%s", password))
		}
	}
	return env, nil
}

func runCommand(command string, args []string, env []string) (int, error) {

	var cmd = exec.Command(command, args...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	err := cmd.Run()
	switch err := err.(type) {
	case nil:
		return 0, nil
	case *exec.ExitError:
		return cmd.ProcessState.ExitCode(), nil
	default:
		return 1, err
	}
}

func searchForUsername(args []string) string {
	var username = searchForUsernameInArgs(args)
	if username == "" {
		username = os.Getenv("PGUSER")
	}
	return username
}

func isShortOption(arg string) bool {
	return strings.HasPrefix(arg, "-")
}

func isLongOption(arg string) bool {
	return strings.HasPrefix(arg, "--")
}

var shortOptionsHavingArg = map[byte]bool{
	'c': true,
	'd': true,
	'f': true,
	'V': true,
	'?': true,
	'F': true,
	'R': true,
	'T': true,
	'h': true,
	'p': true,
	'U': true,
}

var longOptionsHavingArg = map[string]bool{
	"command":          true,
	"dbname":           true,
	"file":             true,
	"set":              true,
	"variable":         true,
	"help":             true,
	"log-file":         true,
	"output":           true,
	"field-separator":  true,
	"pset":             true,
	"record-separator": true,
	"table-attr":       true,
	"host":             true,
	"port":             true,
	"username":         true,
}

func searchForUsernameInArgs(args []string) string {
	var username string

	for i := 0; i < len(args); i++ {

		arg := args[i]

		if isLongOption(arg) {

			if len(arg) <= 2 {
				continue
			}

			var value string
			kv := strings.SplitN(arg[2:], "=", 2)
			longName := kv[0]
			if len(kv) >= 2 {
				value = kv[1]
			} else if longOptionsHavingArg[longName] {
				if i+1 < len(args) {
					i++
					value = args[i]
				}
			}

			if longName == "username" {
				username = value
			}

		} else if isShortOption(arg) {

			if len(arg) <= 1 {
				continue
			}

			shortName := arg[1]

			var value string
			if len(arg) > 2 {
				value = arg[2:]
			} else if shortOptionsHavingArg[shortName] {
				if i+1 < len(args) {
					i++
					value = args[i]
				}
			}

			if shortName == 'U' {
				username = value
			}

		} else {
			found := searchForUsernameInNonOptionArg(arg)
			if found != "" {
				username = found
			}
		}
	}

	return username
}

func searchForUsernameInNonOptionArg(arg string) string {
	if strings.HasPrefix(arg, "postgresql:") {
		return searchForUsernameInConnectionURI(arg)
	} else {
		return searchForUsernameInConnectionString(arg)
	}
}

func searchForUsernameInConnectionURI(uri string) string {
	var u, err = url.Parse(uri)
	if err != nil {
		fmt.Fprintf(os.Stderr, "wrapper: %v\n", err)
		return ""
	}
	return u.User.Username()
}

func searchForUsernameInConnectionString(s string) string {
	var re = regexp.MustCompile(`\s+`)
	var params = re.Split(s, -1)
	for _, param := range params {
		var kv = strings.SplitN(param, "=", 2)
		if len(kv) == 2 && kv[0] == "user" {
			return strings.TrimSpace(kv[1])
		}
	}
	return ""
}

func retrievePasswordForUser(username string) (string, error) {
	var provider = getPasswordProvider()
	if provider == "" {
		return "", errors.New("environment variable PGW_PASSWORD_PROVIDER is undefined")
	}
	return invokePasswordProvider(provider, username)
}

func getPasswordProvider() string {
	return os.Getenv("PGW_PASSWORD_PROVIDER")
}

func invokePasswordProvider(provider string, username string) (string, error) {
	var cmd = exec.Command(provider, username)
	var stdout, err = cmd.Output()
	switch err := err.(type) {
	case nil:
		// Removes trailing new lines
		password := strings.TrimRight(string(stdout), "\n")
		return password, nil
	case *exec.ExitError:
		return "", fmt.Errorf("password provider \"%s\" exited with an error: %w", provider, err)
	default:
		return "", fmt.Errorf("failed to invoke the password provider: %w", err)
	}
}
