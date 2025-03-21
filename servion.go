/*
 * Copyright (c) 2025 Karagatan LLC.
 * SPDX-License-Identifier: BUSL-1.1
 */

package servion

import (
	"fmt"
	"go.arpabet.com/glue"
	"os"
)

func doRun(scanner glue.Scanner) (err error) {
	context, err := glue.New(ZapLogFactory(), scanner)
	if err != nil {
		return err
	}
	defer context.Close()

	return nil
}

func Run(scanner glue.Scanner) {
	if err := doRun(scanner); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
