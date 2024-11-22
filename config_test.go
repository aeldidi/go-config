package config_test

import (
	"strings"
	"testing"

	"go.eldidi.org/config"
)

func TestSimpleReflect(t *testing.T) {
	var conf struct {
		Cool string
	}
	err := config.Read("<input>", strings.NewReader(`
	`), &conf)
	if err == nil {
		t.Fatal("expected error, found no error")
	}

	err = config.Read("<input>", strings.NewReader(`
	cool = beans
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse config into struct: %v", err)
	}

	if conf.Cool != "beans" {
		t.Fatalf(`expected "beans", found "%v"`, conf.Cool)
	}

	err = config.Read("<input>", strings.NewReader(`
	cool = beans # comment
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse config into struct: %v", err)
	}

	if conf.Cool != "beans" {
		t.Fatalf(`expected "beans", found "%v"`, conf.Cool)
	}
}

func TestRenameReflect(t *testing.T) {
	var conf struct {
		Cool string `config:"coolio"`
	}
	err := config.Read("<input>", strings.NewReader(`
	`), &conf)
	if err == nil {
		t.Fatal("expected error, found no error")
	}

	err = config.Read("<input>", strings.NewReader(`
	coolio = beans
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse config into struct: %v", err)
	}

	if conf.Cool != "beans" {
		t.Fatalf(`expected "beans", found "%v"`, conf.Cool)
	}

	err = config.Read("<input>", strings.NewReader(`
	coolio = beans # comment
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse config into struct: %v", err)
	}

	if conf.Cool != "beans" {
		t.Fatalf(`expected "beans", found "%v"`, conf.Cool)
	}
}

func TestOptionalReflect(t *testing.T) {
	var conf struct {
		Cool string `config:"coolio,optional"`
	}
	err := config.Read("<input>", strings.NewReader(`
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse: %v", err)
	}

	err = config.Read("<input>", strings.NewReader(`
	coolio = beans
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse config into struct: %v", err)
	}

	if conf.Cool != "beans" {
		t.Fatalf(`expected "beans", found "%v"`, conf.Cool)
	}

	err = config.Read("<input>", strings.NewReader(`
	coolio = beans # comment
	`), &conf)
	if err != nil {
		t.Fatalf("failed to parse config into struct: %v", err)
	}

	if conf.Cool != "beans" {
		t.Fatalf(`expected "beans", found "%v"`, conf.Cool)
	}
}

func TestEmptyMap(t *testing.T) {
	conf, err := config.Parse("<input>", strings.NewReader(`
	`))
	if err != nil {
		t.Fatal(err)
	}

	if len(conf) != 0 {
		t.Fatalf("empty config parses into values: %v", conf)
	}
}

func TestCommentMap(t *testing.T) {
	conf, err := config.Parse("<input>", strings.NewReader(`
	# comment
	`))
	if err != nil {
		t.Fatal(err)
	}

	if len(conf) != 0 {
		t.Fatalf("empty config parses into values: %v", conf)
	}
}

func TestKeyValueMap(t *testing.T) {
	conf, err := config.Parse("<input>", strings.NewReader(`
	cool = beans
	`))
	if err != nil {
		t.Fatal(err)
	}

	if len(conf) != 1 {
		t.Fatalf("expected 1 value, found %v: %v", len(conf), conf)
	}

	value, ok := conf["cool"]
	if !ok {
		t.Fatalf(`expected "beans", found "%v"`, value)
	}
}

func TestKeyValueCommentMap(t *testing.T) {
	conf, err := config.Parse("<input>", strings.NewReader(`
	cool = beans # comment
	`))
	if err != nil {
		t.Fatal(err)
	}

	if len(conf) != 1 {
		t.Fatalf("expected 1 value, found %v: %v", len(conf), conf)
	}

	value, ok := conf["cool"]
	if !ok {
		t.Fatalf(`expected "beans", found "%v"`, value)
	}
}
