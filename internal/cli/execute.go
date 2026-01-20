package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
)

// Execute runs the CLI with the provided args and manager.
func Execute(args []string, manager Manager, out, errOut io.Writer) int {
	cmd := NewRootCommand(manager, out, errOut)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		var usageErr *usageError
		if errors.As(err, &usageErr) {
			return ExitInvalidUsage
		}
		return ExitRuntimeError
	}
	return ExitSuccess
}

// NewRootCommand builds the root CLI command tree.
func NewRootCommand(manager Manager, out, errOut io.Writer) *cobra.Command {
	root := &cobra.Command{
		Use:           "ontree",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.SetOut(out)
	root.SetErr(errOut)

	root.PersistentFlags().Bool("json", false, "output JSONL")

	root.AddCommand(newSetupCommand(manager))
	root.AddCommand(newAppCommand(manager))
	root.AddCommand(newModelCommand(manager))

	return root
}

type usageError struct {
	err error
}

func (u *usageError) Error() string {
	if u.err == nil {
		return "invalid usage"
	}
	return u.err.Error()
}

func requireArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return &usageError{err: fmt.Errorf("requires %d argument(s)", n)}
		}
		return nil
	}
}

func newSetupCommand(manager Manager) *cobra.Command {
	setup := &cobra.Command{
		Use:   "setup",
		Short: "initial setup",
	}

	initCmd := &cobra.Command{
		Use:   "init",
		Short: "initialize admin user and node settings",
		RunE: func(cmd *cobra.Command, _ []string) error {
			username, _ := cmd.Flags().GetString("username")
			password, _ := cmd.Flags().GetString("password")
			nodeName, _ := cmd.Flags().GetString("node-name")
			nodeIcon, _ := cmd.Flags().GetString("node-icon")
			if username == "" || password == "" {
				return &usageError{err: fmt.Errorf("username and password are required")}
			}

			if err := manager.SetupInit(cmd.Context(), username, password, nodeName, nodeIcon); err != nil {
				return writeError(cmd, err)
			}
			return writeEvent(cmd, ProgressEvent{Type: "success", Message: "setup complete"})
		},
	}
	initCmd.Flags().String("username", "", "admin username")
	initCmd.Flags().String("password", "", "admin password")
	initCmd.Flags().String("node-name", "", "node name")
	initCmd.Flags().String("node-icon", "", "node icon")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "check setup status",
		RunE: func(cmd *cobra.Command, _ []string) error {
			status, err := manager.SetupStatus(cmd.Context())
			if err != nil {
				return writeError(cmd, err)
			}
			return writeEvent(cmd, ProgressEvent{Type: "result", Data: status})
		},
	}

	setup.AddCommand(initCmd, statusCmd)
	return setup
}

func newAppCommand(manager Manager) *cobra.Command {
	app := &cobra.Command{
		Use:   "app",
		Short: "manage apps",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list apps",
		RunE: func(cmd *cobra.Command, _ []string) error {
			apps, err := manager.AppList(cmd.Context())
			if err != nil {
				return writeError(cmd, err)
			}
			return writeEvent(cmd, ProgressEvent{Type: "result", Data: apps})
		},
	}

	installCmd := &cobra.Command{
		Use:   "install <app>",
		Short: "install an app",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			version, _ := cmd.Flags().GetString("version")
			envPath, _ := cmd.Flags().GetString("env")
			return streamEvents(cmd, manager.AppInstall(cmd.Context(), args[0], version, envPath))
		},
	}
	installCmd.Flags().String("version", "", "template version")
	installCmd.Flags().String("env", "", "env file path")

	startCmd := &cobra.Command{
		Use:   "start <app>",
		Short: "start an app",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return streamEvents(cmd, manager.AppStart(cmd.Context(), args[0]))
		},
	}

	stopCmd := &cobra.Command{
		Use:   "stop <app>",
		Short: "stop an app",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return streamEvents(cmd, manager.AppStop(cmd.Context(), args[0]))
		},
	}

	healthCmd := &cobra.Command{
		Use:   "health <app>",
		Short: "check app health",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			httpURL, _ := cmd.Flags().GetString("http")
			timeout, _ := cmd.Flags().GetDuration("timeout")
			interval, _ := cmd.Flags().GetDuration("interval")
			return streamEvents(cmd, manager.AppHealth(cmd.Context(), args[0], httpURL, timeout, interval))
		},
	}
	healthCmd.Flags().String("http", "", "http url to probe")
	healthCmd.Flags().Duration("timeout", 180*time.Second, "timeout for health checks")
	healthCmd.Flags().Duration("interval", 3*time.Second, "interval for health checks")

	app.AddCommand(listCmd, installCmd, startCmd, stopCmd, healthCmd)
	return app
}

func newModelCommand(manager Manager) *cobra.Command {
	model := &cobra.Command{
		Use:   "model",
		Short: "manage models",
	}

	listCmd := &cobra.Command{
		Use:   "list",
		Short: "list models",
		RunE: func(cmd *cobra.Command, _ []string) error {
			models, err := manager.ModelList(cmd.Context())
			if err != nil {
				return writeError(cmd, err)
			}
			return writeEvent(cmd, ProgressEvent{Type: "result", Data: models})
		},
	}

	installCmd := &cobra.Command{
		Use:   "install <model>",
		Short: "install a model",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return streamEvents(cmd, manager.ModelInstall(cmd.Context(), args[0]))
		},
	}

	healthCmd := &cobra.Command{
		Use:   "health <model>",
		Short: "check model health",
		Args:  requireArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, _ := cmd.Flags().GetDuration("timeout")
			interval, _ := cmd.Flags().GetDuration("interval")
			return streamEvents(cmd, manager.ModelHealth(cmd.Context(), args[0], timeout, interval))
		},
	}
	healthCmd.Flags().Duration("timeout", 180*time.Second, "timeout for health checks")
	healthCmd.Flags().Duration("interval", 3*time.Second, "interval for health checks")

	model.AddCommand(listCmd, installCmd, healthCmd)
	return model
}

func streamEvents(cmd *cobra.Command, events <-chan ProgressEvent) error {
	ctx := cmd.Context()
	jsonOutput, _ := cmd.Flags().GetBool("json")
	hasError := false
	for event := range events {
		if err := writeEventWithContext(ctx, cmd, event, jsonOutput); err != nil {
			return err
		}
		if event.Type == "error" {
			hasError = true
		}
	}
	if hasError {
		return &runtimeError{err: fmt.Errorf("operation failed")}
	}
	return nil
}

type runtimeError struct {
	err error
}

func (r *runtimeError) Error() string {
	if r.err == nil {
		return "runtime error"
	}
	return r.err.Error()
}

func writeError(cmd *cobra.Command, err error) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	if jsonOutput {
		_ = writeEventWithContext(cmd.Context(), cmd, ProgressEvent{
			Type:    "error",
			Message: err.Error(),
		}, true)
	}
	return &runtimeError{err: err}
}

func writeEvent(cmd *cobra.Command, event ProgressEvent) error {
	jsonOutput, _ := cmd.Flags().GetBool("json")
	return writeEventWithContext(cmd.Context(), cmd, event, jsonOutput)
}

func writeEventWithContext(ctx context.Context, cmd *cobra.Command, event ProgressEvent, jsonOutput bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if jsonOutput {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		return encoder.Encode(event)
	}
	if event.Message != "" {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), event.Message)
		return err
	}
	return nil
}
