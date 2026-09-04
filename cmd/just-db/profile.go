package main

import (
	"flag"
	"fmt"
	"strings"

	"just-db/internal/appmeta"
	"just-db/internal/engine"
	"just-db/internal/profile"
	"just-db/internal/registry"
)

func cmdProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: %s profile <list|save|show|delete>", appmeta.Name)
	}
	switch args[0] {
	case "list":
		return cmdProfileList(args[1:])
	case "save":
		return cmdProfileSave(args[1:])
	case "show":
		return cmdProfileShow(args[1:])
	case "delete":
		return cmdProfileDelete(args[1:])
	default:
		return fmt.Errorf("unknown profile command %q", args[0])
	}
}

func cmdProfileList(args []string) error {
	fs := flag.NewFlagSet("profile list", flag.ContinueOnError)
	data := registerDataFlag(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	st, err := profile.Open(*data)
	if err != nil {
		return err
	}
	list, err := st.List()
	if err != nil {
		return err
	}
	if len(list) == 0 {
		fmt.Println("no profiles")
		return nil
	}
	for _, item := range list {
		db := item.Database
		if db == "" {
			db = "-"
		}
		fmt.Printf("%s  %s  %s:%d  %s / %s\n", item.Name, item.Engine, item.Host, item.Port, item.User, db)
	}
	return nil
}

func cmdProfileSave(args []string) error {
	fs := flag.NewFlagSet("profile save", flag.ContinueOnError)
	data := registerDataFlag(fs)
	name := fs.String("name", "", "profile name")
	c := registerConnFlags(fs)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("-name is required")
	}
	if _, err := registry.New().Get(c.engine()); err != nil {
		return err
	}
	st, err := profile.Open(*data)
	if err != nil {
		return err
	}
	rec := profile.Record{Name: *name, Engine: c.engine(), Connection: c.connection()}
	if err := st.Put(rec); err != nil {
		return err
	}
	fmt.Printf("saved %s\n", rec.Name)
	return nil
}

func cmdProfileShow(args []string) error {
	fs := flag.NewFlagSet("profile show", flag.ContinueOnError)
	data := registerDataFlag(fs)
	name := fs.String("name", "", "profile name")
	includePassword := fs.Bool("include-password", false, "print the stored password")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("-name is required")
	}
	st, err := profile.Open(*data)
	if err != nil {
		return err
	}
	rec, err := st.Get(*name)
	if err != nil {
		return err
	}
	password := "(hidden)"
	if *includePassword {
		password = rec.Connection.Password
	}
	fmt.Printf("name      %s\n", rec.Name)
	fmt.Printf("engine    %s\n", rec.Engine)
	fmt.Printf("host      %s\n", rec.Connection.Host)
	fmt.Printf("port      %d\n", rec.Connection.Port)
	fmt.Printf("user      %s\n", rec.Connection.User)
	fmt.Printf("password  %s\n", password)
	fmt.Printf("database  %s\n", rec.Connection.Database)
	fmt.Printf("sslmode   %s\n", rec.Connection.SSLMode)
	return nil
}

func cmdProfileDelete(args []string) error {
	fs := flag.NewFlagSet("profile delete", flag.ContinueOnError)
	data := registerDataFlag(fs)
	name := fs.String("name", "", "profile name")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *name == "" {
		return fmt.Errorf("-name is required")
	}
	st, err := profile.Open(*data)
	if err != nil {
		return err
	}
	if err := st.Delete(*name); err != nil {
		return err
	}
	fmt.Printf("deleted %s\n", *name)
	return nil
}

func registerDataFlag(fs *flag.FlagSet) *string {
	return fs.String("data", envOr("JUSTDB_DATA", "./data"), "data directory for dumps and profiles")
}

func registerProfileFlag(fs *flag.FlagSet) *string {
	return fs.String("profile", "", "saved profile name (flags override stored fields)")
}

func resolveConn(fs *flag.FlagSet, c connFlags, dataDir, profileName string) (engine.Name, engine.Connection, error) {
	if strings.TrimSpace(profileName) == "" {
		return c.engine(), c.connection(), nil
	}
	st, err := profile.Open(dataDir)
	if err != nil {
		return "", engine.Connection{}, err
	}
	rec, err := st.Get(profileName)
	if err != nil {
		return "", engine.Connection{}, err
	}
	eng := rec.Engine
	cfg := rec.Connection
	fs.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "engine":
			eng = c.engine()
		case "host":
			cfg.Host = *c.host
		case "port":
			cfg.Port = *c.port
		case "user":
			cfg.User = *c.user
		case "password":
			cfg.Password = *c.password
		case "database":
			cfg.Database = *c.database
		case "sslmode":
			cfg.SSLMode = *c.sslmode
		}
	})
	if cfg.Password == "" {
		cfg.Password = c.connection().Password
	}
	return eng, cfg, nil
}
