package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/bspeelm/bothy/internal/config"
)

// `bothy config` -- read and write the handful of choices bothy keeps.

func cmdConfig(args []string) error {
	p, cfg, err := load()
	if err != nil {
		return err
	}
	if len(args) == 0 {
		args = []string{"get"}
	}

	switch args[0] {
	case "path":
		fmt.Println(config.Path(p))
	case "get":
		out, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(out))
	case "set":
		if len(args) != 3 {
			return fmt.Errorf("usage: bothy config set <key> <value>")
		}
		if err := cfg.Set(args[1], args[2]); err != nil {
			return err
		}
		if err := config.Save(p, cfg); err != nil {
			return err
		}
		fmt.Printf("set %s = %s\n", args[1], args[2])
		if err := cfg.Validate(); err != nil {
			fmt.Printf("\nnot ready yet:\n%v\n", err)
			return nil
		}
		fmt.Println("run 'bothy install' to apply it")
	case "edit":
		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vi"
		}
		if err := config.Save(p, cfg); err != nil { // ensure the file exists
			return err
		}
		return runInteractive(editor, config.Path(p))
	default:
		return fmt.Errorf("usage: bothy config [get|set|edit|path]")
	}
	return nil
}
