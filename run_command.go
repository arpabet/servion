/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"fmt"
	"github.com/pkg/errors"
	"go.arpabet.com/cligo"
	"go.arpabet.com/glue"
	"go.uber.org/zap"
)

type implRunCommand struct {
	Parent  cligo.CliGroup `cli:"group=cli"`
	HomeDir string         `cli:"option=home,default=.,help=home directory of application"`
	Bind    string         `cli:"option=bind,default=,help=bind listening address"`
	beans   []interface{}
}

func RunCommand(scan ...interface{}) cligo.CliCommand {
	return &implRunCommand{beans: scan}
}

func (cmd *implRunCommand) Command() string {
	return "run"
}

func (cmd *implRunCommand) Help() (string, string) {
	return "Runs the server.",
		`This command runs the server in foreground and prints the results to standard output.
This command accepts one argument that is profile, that helps to define how the server is running.`
}

func (cmd *implRunCommand) Run(ctx glue.Context) (err error) {

runItAgain:

	beans := make([]interface{}, len(cmd.beans))
	copy(beans, cmd.beans)

	// always new runtime
	runtime := NewRuntime(cmd.HomeDir)
	beans = append(beans, runtime)

	var logger *zap.Logger
	zapBeans := ctx.Bean(ZapLogClass, glue.DefaultLevel)
	if len(zapBeans) == 0 {
		logger, err = zap.NewDevelopment()
		if err != nil {
			return errors.Errorf("failed to initialize zap logger: %v", err)
		}
		beans = append(beans, logger)
	} else {
		logger = zapBeans[0].Object().(*zap.Logger)
	}

	child, err := ctx.Extend(beans...)
	if err != nil {
		return fmt.Errorf("fail to initialize '%s' command scope context, %v", cmd.Command(), err)
	}

	err = runServers(runtime, child, logger)
	if err != nil {
		logger.Error("RunServers", zap.Bool("restarting", runtime.Restarting()), zap.Error(err))
	} else {
		logger.Info("RunServers", zap.Bool("restarting", runtime.Restarting()))
	}

	err = child.Close()
	if err != nil {
		logger.Error("ChildContextClose", zap.Error(err))
	}

	if runtime.Restarting() {
		goto runItAgain
	}

	return nil
}
