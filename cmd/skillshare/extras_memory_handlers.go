package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"skillshare/internal/config"
	"skillshare/internal/memory"
	"skillshare/internal/oplog"
	"skillshare/internal/ui"
)

func runMemory(mode runMode, opts memoryOptions) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	mode = resolveAutoMode(mode, cwd)
	var root, cfgPath, projectRoot string
	var initialize func() error
	if mode == modeProject {
		cfg, err := config.LoadProject(cwd)
		if err != nil {
			return err
		}
		root, err = memory.ProjectRoot(cfg, cwd)
		if err != nil {
			return err
		}
		cfgPath, projectRoot = config.ProjectConfigPath(cwd), cwd
		initialize = func() error {
			if err := memory.Init(root); err != nil {
				return err
			}
			extra, found, err := memory.Extra(cfg.Extras)
			if err != nil || found {
				return err
			}
			cfg.Extras = append(cfg.Extras, extra)
			return cfg.Save(cwd)
		}
	} else {
		cfg, err := config.Load()
		if err != nil {
			return err
		}
		root, err = memory.GlobalRoot(cfg)
		if err != nil {
			return err
		}
		cfgPath = config.ConfigPath()
		initialize = func() error {
			if err := memory.Init(root); err != nil {
				return err
			}
			extra, found, err := memory.Extra(cfg.Extras)
			if err != nil || found {
				return err
			}
			cfg.Extras = append(cfg.Extras, extra)
			return cfg.Save()
		}
	}
	start := time.Now()
	if opts.command == "init" || opts.command == "write" || opts.command == "delete" {
		defer func() {
			entry := oplog.NewEntry("memory-"+opts.command, statusFromErr(err), time.Since(start))
			entry.Args = map[string]any{"path": opts.path, "scope": modeString(mode)}
			oplog.WriteWithLimit(cfgPath, oplog.OpsFile, entry, logMaxEntries()) //nolint:errcheck
		}()
	}
	var result any
	switch opts.command {
	case "init":
		err = initialize()
		result = map[string]any{"root": root}
	case "list":
		var notes []memory.Note
		notes, err = memory.List(root, opts.search)
		result = map[string]any{"root": root, "notes": notes}
		if err == nil && !opts.json {
			for _, note := range notes {
				fmt.Printf("%s\t%s\n", note.Path, note.Title)
			}
			return nil
		}
	case "show":
		var note memory.Note
		note, err = memory.Read(root, opts.path)
		result = note
		if err == nil && !opts.json {
			fmt.Print(note.Content)
			return nil
		}
	case "write":
		var input io.ReadCloser = os.Stdin
		if opts.from != "-" {
			input, err = os.Open(opts.from)
			if err != nil {
				return err
			}
			defer input.Close()
		}
		var data []byte
		data, err = io.ReadAll(io.LimitReader(input, memory.MaxNoteBytes+1))
		if err != nil {
			return err
		}
		result, err = memory.Write(root, opts.path, string(data), opts.version)
	case "delete":
		err = memory.Delete(root, opts.path, opts.version)
		result = map[string]any{"success": err == nil, "path": opts.path}
	case "instructions":
		text := memory.Instructions(root, projectRoot, opts.mode)
		if !opts.json {
			fmt.Print(text)
			return nil
		}
		result = map[string]any{"root": root, "mode": opts.mode, "instructions": text}
	}
	if err != nil {
		return err
	}
	if opts.json {
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	switch opts.command {
	case "init":
		ui.Done(ui.MarkOK, "Memory ready at "+shortenPath(root), 0)
	case "write":
		ui.Done(ui.MarkOK, "Saved "+opts.path, 0)
	case "delete":
		ui.Done(ui.MarkOK, "Deleted "+opts.path, 0)
	}
	return nil
}
