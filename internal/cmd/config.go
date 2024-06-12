package cmd

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"

	configPkg "github.com/nobe4/gh-not/internal/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	editConfigFlag = false
	initConfigFlag = false

	configCmd = &cobra.Command{
		Use:   "config",
		Short: "Print the config to stdout",
		RunE:  runConfig,
	}
)

func init() {
	rootCmd.AddCommand(configCmd)

	configCmd.Flags().BoolVarP(&editConfigFlag, "edit", "e", false, "Edit the config in $EDITOR")
	configCmd.Flags().BoolVarP(&initConfigFlag, "init", "i", false, "Print the default config to stdout")
}

func runConfig(cmd *cobra.Command, args []string) error {
	if initConfigFlag {
		return initConfig()
	}

	if editConfigFlag {
		return editConfig()
	}

	marshalled, err := yaml.Marshal(config)
	if err != nil {
		slog.Error("Failed to marshall config", "err", err)
		return err
	}

	fmt.Printf("Config sourced from: %s\n\n%s\n", configPathFlag, marshalled)

	return nil
}

func initConfig() error {
	slog.Debug("printing config file", "path", configPathFlag)

	marshalled, err := yaml.Marshal(configPkg.Default())
	if err != nil {
		slog.Error("Failed to marshall config", "err", err)
		return err
	}

	fmt.Printf("%s\n", marshalled)

	return nil
}

func editConfig() error {
	slog.Debug("editing config file", "path", configPathFlag)

	editor := os.Getenv("EDITOR")
	if editor == "" {
		return fmt.Errorf("EDITOR environment variable not set")
	}

	cmd := exec.Command(editor, configPathFlag)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if err := cmd.Wait(); err != nil {
		return err
	}

	return nil
}
