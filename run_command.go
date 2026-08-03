/*
 * Copyright (c) 2026 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"context"

	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.uber.org/zap"
	"golang.org/x/xerrors"
)

type implRunCommand struct {
	Parent  cligo.CliGroup `cli:"group=cli"`
	HomeDir string         `cli:"option=home,default=.,help=home directory of application"`
	Bind    string         `cli:"option=bind,default=,help=bind listening address"`
	beans   []interface{}

	Container glue.Container `inject:""`
}

func RunCommand(scan ...interface{}) cligo.CliCommand {
	return &implRunCommand{beans: scan}
}

func (t *implRunCommand) Command() string {
	return "run"
}

func (t *implRunCommand) Help() (string, string) {
	return "Runs the server.",
		`This command runs the server in foreground and prints the results to standard output.
This command accepts one argument that is profile, that helps to define how the server is running.`
}

func (t *implRunCommand) Run(ctx context.Context) (err error) {

	beans := make([]interface{}, len(t.beans))
	copy(beans, t.beans)

	// always new runtime
	runtime := NewRuntime(t.HomeDir)
	beans = append(beans, runtime)

	var logger *zap.Logger
	zapBeans := t.Container.Bean(ZapLogClass, glue.DefaultSearchLevel)
	if len(zapBeans) == 0 {
		logger, err = zap.NewDevelopment()
		if err != nil {
			return xerrors.Errorf("failed to initialize zap logger: %w", err)
		}
		beans = append(beans, logger)
	} else {
		logger = zapBeans[0].Object().(*zap.Logger)
	}

	child, err := t.Container.Extend(beans...)
	if err != nil {
		return xerrors.Errorf("failed to initialize '%s' command scope context: %w", t.Command(), err)
	}

	err = runServers(runtime, child, logger)
	if err != nil {
		logger.Error("RunServersDone", zap.Bool("restarting", runtime.Restarting()), zap.Error(err))
	} else {
		logger.Info("RunServersDone", zap.Bool("restarting", runtime.Restarting()))
	}

	if e := child.Close(); e != nil {
		logger.Error("ChildContextClose", zap.Error(e))
	}

	if runtime.Restarting() {
		// Delegate the restart to cligo: it rebuilds a fresh container with the
		// config re-read and profiles re-resolved (e.g. onboarding → prod),
		// without exiting the OS process — so a Kubernetes pod is not restarted
		// and a stateful store's lock is released with the closed child context.
		return cligo.ErrRepeatRun
	}

	/*
		The run's error IS the command's error. It used to be logged above and
		then discarded, which made the two failures a person most needs to see
		exit 0 with nothing on the terminal: a server whose every bind failed (a
		second instance of the application — banner, prompt, gone), and a server
		whose Serve crashed. The log line stays for whoever tails the file; the
		return is for cligo, which prints it to stderr and sets the exit code —
		error handling lives in the framework, not in application os.Exit calls.

		A clean shutdown still returns nil: Serve's closed-socket errors are
		filtered by the server implementations, so g.Wait is nil on SIGTERM.
	*/
	return err
}
