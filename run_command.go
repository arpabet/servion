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
	"go.arpabet.com/servion/servionapi"
	"go.uber.org/zap"
)

type implRunCommand struct {
	Parent  cligo.CliGroup `cli:"group=cli"`
	Profile string         `cli:"argument=profile,default=dev,help=execution profile"`
	HomeDir string         `cli:"option=home,default=.,help=home directory of application"`
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
	
	runtime := NewRuntime(cmd.Profile, cmd.HomeDir)
	cmd.beans = append(cmd.beans, runtime)

	var logger *zap.Logger
	zapBeans := ctx.Bean(servionapi.ZapLogClass, glue.DefaultLevel)
	if len(zapBeans) == 0 {
		logger, err = zap.NewDevelopment()
		if err != nil {
			return errors.Errorf("failed to initialize zap logger: %v", err)
		}
		cmd.beans = append(cmd.beans, logger)
	} else {
		logger = zapBeans[0].Object().(*zap.Logger)
	}

	child, err := ctx.Extend(cmd.beans...)
	if err != nil {
		return fmt.Errorf("fail to initialize '%s' command scope context, %v", cmd.Command(), err)
	}
	defer child.Close()

	return runServers(runtime, child, logger)
}
