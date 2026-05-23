package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

type jsonErrorOutput struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type renderedCommandError struct {
	err error
}

func (e renderedCommandError) Error() string {
	if e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e renderedCommandError) Unwrap() error {
	return e.err
}

func wantsJSON(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}

	flag := cmd.Flags().Lookup("json")
	if flag == nil {
		flag = cmd.InheritedFlags().Lookup("json")
	}
	if flag == nil {
		return false
	}

	enabled, err := strconv.ParseBool(flag.Value.String())
	if err != nil {
		return false
	}

	return enabled
}

func writeJSONOutput(cmd *cobra.Command, payload any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

func writeCompactJSONOutput(cmd *cobra.Command, payload any) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(payload)
}

func writeJSONError(cmd *cobra.Command, err error) error {
	if err == nil {
		return nil
	}

	if wantsJSON(cmd) {
		encodeErr := json.NewEncoder(cmd.ErrOrStderr()).Encode(jsonErrorOutput{
			OK:    false,
			Error: err.Error(),
		})
		if encodeErr != nil {
			_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "failed to encode JSON error: %v\n", encodeErr)
			return renderedCommandError{err: err}
		}
		return renderedCommandError{err: err}
	}

	return err
}

func commandRequestedJSON(cmd *cobra.Command, args []string) bool {
	if wantsJSON(cmd) {
		return true
	}

	found := false
	enabled := false
	for _, arg := range args {
		switch {
		case arg == "--json":
			found = true
			enabled = true
		case strings.HasPrefix(arg, "--json="):
			parsed, err := strconv.ParseBool(strings.TrimPrefix(arg, "--json="))
			if err != nil {
				continue
			}
			found = true
			enabled = parsed
		}
	}

	if found {
		return enabled
	}

	return false
}

func executeRootCommand(root *cobra.Command, args []string) error {
	if root == nil {
		return nil
	}

	root.SilenceErrors = true
	if args == nil {
		root.SetArgs([]string{})
	} else {
		root.SetArgs(args)
	}

	err := root.Execute()
	if err == nil {
		return nil
	}

	var rendered renderedCommandError
	if errors.As(err, &rendered) {
		return rendered.Unwrap()
	}

	if commandRequestedJSON(root, args) {
		encodeErr := json.NewEncoder(root.ErrOrStderr()).Encode(jsonErrorOutput{
			OK:    false,
			Error: err.Error(),
		})
		if encodeErr != nil {
			_, _ = fmt.Fprintf(root.ErrOrStderr(), "failed to encode JSON error: %v\n", encodeErr)
			return err
		}
		return err
	}

	if _, printErr := fmt.Fprintf(root.ErrOrStderr(), "Error: %v\n", err); printErr != nil {
		return printErr
	}

	return err
}
