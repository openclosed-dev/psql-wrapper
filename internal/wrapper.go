package internal

import (
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

type wrapper struct {
	name   string
	logger *log.Logger
	path   string
}

const defaultPasswordProvider = "password_provider"

func Launch(name string, command string, args []string) int {

	var w = wrapper{
		name:   name,
		logger: log.New(os.Stderr, name+": ", 0),
		path:   args[0],
	}

	return w.launch(command, args[1:])
}

func (w *wrapper) launch(command string, args []string) int {

	env, err := w.buildEnv(args)
	if err != nil {
		w.logger.Println(err)
		return 1
	}

	exitCode, err := w.runCommand(command, args, env)
	if err != nil {
		w.logger.Println(err)
	}

	return exitCode
}

func (w *wrapper) buildEnv(args []string) ([]string, error) {
	var env = os.Environ()
	var username = w.searchForUsername(args)
	if username == "" {
		w.logger.Printf("Cannot detect username to login")
	} else {
		var password, err = w.retrievePasswordForUser(username)
		if err != nil {
			return env, err
		}
		if password != "" {
			env = append(env, fmt.Sprintf("PGPASSWORD=%s", password))
		}
	}
	return env, nil
}

func (w *wrapper) runCommand(command string, args []string, env []string) (int, error) {

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

func (w *wrapper) searchForUsername(args []string) string {
	var username = w.searchArgsForUsername(args)
	if username == "" {
		username = os.Getenv("PGUSER")
	}
	return username
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

func (w *wrapper) searchArgsForUsername(args []string) string {
	var username string
	var positional []string

	for i := 0; i < len(args); i++ {

		var arg string = args[i]

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
			positional = append(positional, arg)
		}
	}

	if found := w.searchPositionalArgsForUsername(positional); found != "" {
		username = found
	}

	return username
}

func (w *wrapper) searchPositionalArgsForUsername(args []string) string {
	var len = len(args)
	switch len {
	case 0:
		return ""
	case 1:
		return w.searchConnectionArgForUsername(args[0])
	case 2:
		return args[1]
	default:
		w.logger.Printf("Too many positional arguments: %d", len)
		return ""
	}
}

func (w *wrapper) searchConnectionArgForUsername(arg string) string {
	if strings.HasPrefix(arg, "postgresql:") {
		return w.searchConnectionURIForUsername(arg)
	} else {
		return w.searchConnectionStringForUsername(arg)
	}
}

func (w *wrapper) searchConnectionURIForUsername(uri string) string {
	var u, err = url.Parse(uri)
	if err != nil {
		w.logger.Println(err)
		return ""
	}
	return u.User.Username()
}

func (w *wrapper) searchConnectionStringForUsername(s string) string {
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

func (w *wrapper) retrievePasswordForUser(username string) (string, error) {
	var provider = w.getPasswordProvider()
	if provider == "" {
		return "", errors.New("environment variable PGW_PASSWORD_PROVIDER is undefined")
	}
	return w.invokePasswordProvider(provider, username)
}

func (w *wrapper) invokePasswordProvider(provider string, username string) (string, error) {
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

func (w *wrapper) getPasswordProvider() string {
	var provider = os.Getenv("PGW_PASSWORD_PROVIDER")
	if provider == "" {
		var path = filepath.Join(filepath.Dir(w.path), defaultPasswordProvider)
		if _, err := os.Stat(path); err == nil {
			provider = path
		}
	}
	return provider
}

func isShortOption(arg string) bool {
	return strings.HasPrefix(arg, "-")
}

func isLongOption(arg string) bool {
	return strings.HasPrefix(arg, "--")
}
