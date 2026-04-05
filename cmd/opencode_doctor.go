package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

type doctorCheck struct {
	Name   string
	Status string
	Detail string
}

func newOpencodeDoctorCmd(app *app) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check OpenCode integration",
		RunE: func(cmd *cobra.Command, _ []string) error {
			checks, err := runOpencodeDoctor(cmd.Context(), app)
			if err != nil {
				return err
			}

			for _, check := range checks {
				line := fmt.Sprintf("%s: %s", check.Name, check.Status)
				if check.Detail != "" {
					line += " " + check.Detail
				}
				fmt.Fprintln(cmd.OutOrStdout(), line)
			}

			return nil
		},
	}
}

func runOpencodeDoctor(ctx context.Context, app *app) ([]doctorCheck, error) {
	checks := make([]doctorCheck, 0, 4)

	pluginPath, err := opencodePluginPath()
	if err != nil {
		return nil, err
	}
	checks = append(checks, checkReadableFile("plugin", pluginPath, false))

	checks = append(checks, checkExecutable("oa binary", "oa"))

	authPath, err := opencodeAuthPath()
	if err != nil {
		return nil, err
	}
	checks = append(checks, checkReadableFile("auth file", authPath, false))

	statusChecks, err := app.service.GetStatusAll(ctx)
	if err != nil {
		checks = append(checks, doctorCheck{Name: "account repo", Status: "error", Detail: err.Error()})
		return checks, nil
	}
	_ = statusChecks
	checks = append(checks, doctorCheck{Name: "account repo", Status: "ok"})

	return checks, nil
}

func checkReadableFile(name, path string, required bool) doctorCheck {
	data, err := os.ReadFile(path)
	if err == nil {
		_ = data
		return doctorCheck{Name: name, Status: "ok"}
	}
	if os.IsNotExist(err) {
		if required {
			return doctorCheck{Name: name, Status: "error", Detail: "missing"}
		}
		return doctorCheck{Name: name, Status: "missing"}
	}
	return doctorCheck{Name: name, Status: "error", Detail: err.Error()}
}

func checkExecutable(name, path string) doctorCheck {
	if _, err := exec.LookPath(path); err == nil {
		return doctorCheck{Name: name, Status: "ok"}
	}
	return doctorCheck{Name: name, Status: "error", Detail: "not reachable"}
}
