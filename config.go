// package config allows reading from a simple config file format as well as
// using environment variables as an override.
//
// Here's how it works:
// You have a `struct` somewhere (let's call the type `Config`) which defines
// the different variables your configuration can have, and call [config.Read]
// giving your config object and the filename as arguments.
//
// All the `struct`'s members will be parsed from the config file and from the
// environment variables. Specifically, environment variables override config
// options, and are all uppercase. For example, a config option called `port`
// would be overriden by the `PORT` environment variable.
//
// By default, all struct members are converted to snake_case when added to the
// config file, but this can be overriden using the `config:""` struct tag.
// Note that the name cannot contain commas, and cannot be the word `optional`.
//
// To make something optional in the config, add `optional` to the config
// struct tag. So by itself it would be `config:"optional"`, and with the name
// `shredder` it would be `config:"shredder,optional"` or
// `config:"optional,shredder"` (although the first is preferred).
//
// The `#` character is used as a comment character. Everything after one of
// these is ignored. If you need a value to contain a `#`, you can enclose it
// in single quotes `'` or double quotes `"`.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

// ValueParser is the interface implemented by types that can be parsed from a
// text description of themselves.
type ValueParser interface {
	ParseConfigValue(string) error
}

var (
	ErrInvalid = errors.New("config.Read was not passed a pointer to a struct")
	ErrSyntax  = errors.New("syntax error")
)

type lexer struct {
	left   strings.Builder
	right  strings.Builder
	reader *strings.Reader
	// the character used to start the string, either ' or "
	stringChar rune
	skipLine   bool
	err        error
}

// Skips whitespace, returning any read errors encountered while doing so.
func (l *lexer) skipWhitespace() error {
	for {
		c, _, err := l.reader.ReadRune()
		if err != nil {
			return err
		}

		if !unicode.IsSpace(c) {
			l.reader.UnreadRune()
			return nil
		}
	}
}

func (l *lexer) unexpected(err error) stateFn {
	l.err = fmt.Errorf("an unexpected error occurred: %w", err)
	return nil
}

func (l *lexer) error(err error) stateFn {
	l.err = err
	return nil
}

type stateFn func(l *lexer) stateFn

// Parse parses a configuration file from the given reader into a `map`
// containing each key-value pair given in the file.
func Parse(path string, r io.Reader) (map[string]string, error) {
	result := map[string]string{}
	s := bufio.NewScanner(r)
	lineNo := 1
	for ; s.Scan(); lineNo += 1 {
		text := strings.TrimSpace(s.Text())
		if text == "" {
			continue
		}

		l := lexer{
			reader: strings.NewReader(text),
		}

		for state := beforeEquals; state != nil; {
			state = state(&l)
			if l.err != nil {
				return nil, fmt.Errorf("error:%v:%v: %w", path, lineNo, l.err)
			}
		}

		if l.skipLine {
			continue
		}

		left := strings.TrimSpace(l.left.String())
		right := strings.TrimSpace(l.right.String())

		// An empty left side is not allowed.
		if left == "" {
			return nil, fmt.Errorf(
				"error:%v:%v: left side of assignment empty",
				path, lineNo,
			)
		}
		result[left] = right
	}

	return result, nil
}

// The left hand side of the assignment.
func beforeEquals(l *lexer) stateFn {
	for {
		c, _, err := l.reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				if l.left.Len() > 0 {
					return l.error(errors.New("unexpected identifier"))
				}
				return nil
			}

			// This can't happen, but just in case Go updates the
			// StringReader to return an error here I'll return an
			// unexpected error.
			return l.unexpected(err)
		}
		switch c {
		case '=':
			l.skipWhitespace()
			tmp, _, err := l.reader.ReadRune()
			if err != nil {
				if err == io.EOF {
					return nil
				}

				return l.unexpected(err)
			}

			if tmp == '"' || tmp == '\'' {
				l.stringChar = tmp
				return afterEqualsString
			} else {
				l.reader.UnreadRune()
				return afterEquals
			}
		case '#':
			if l.left.Len() > 0 {
				return l.error(errors.New("unexpected identifier"))
			}

			l.skipLine = true
			return nil
		default:
			l.left.WriteRune(c)
		}
	}
}

// The right hand side of the assignment, no string delimiter.
func afterEquals(l *lexer) stateFn {
	for {
		c, _, err := l.reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			// This can't happen, but just in case Go updates the
			// StringReader to return an error here I'll return an
			// unexpected error.
			return l.unexpected(err)
		}

		if c == '#' {
			return nil
		}

		l.right.WriteRune(c)
	}
}

func afterEqualsString(l *lexer) stateFn {
	for {
		c, _, err := l.reader.ReadRune()
		if err != nil {
			if err == io.EOF {
				return nil
			}

			// This can't happen, but just in case Go updates the
			// StringReader to return an error here I'll return an
			// unexpected error.
			return l.unexpected(err)
		}

		if c == l.stringChar {
			break
		}

		l.right.WriteRune(c)
	}

	// Nothing is valid after the string except whitespace and
	// optionally a comment.

	l.skipWhitespace()
	ch, _, err := l.reader.ReadRune()
	if err == io.EOF || ch == '#' {
		return nil
	}

	if err != nil {
		// This can't happen, but just in case Go updates the
		// StringReader to return an error here I'll return an
		// unexpected error.
		return l.unexpected(err)
	}

	return nil
}

const (
	noField            = "error parsing config '%v': required value %v not present"
	errorParsingConfig = "error parsing config '%v': %w"
	overflow           = "value '%v' would overflow type"
	unsupported        = "attempted to parse unsupported type '%v' (hint: it doesn't implement config.ValueParser)"
)

// Read parses a configuration file at the given path into a struct.
func Read(path string, r io.Reader, obj any) error {
	vals, err := Parse(path, r)
	if err != nil {
		return err
	}

	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Pointer || v.IsNil() {
		return ErrInvalid
	}
	v = reflect.Indirect(v)
	if v.Kind() != reflect.Struct {
		return ErrInvalid
	}

	numFields := v.NumField()
	for i := 0; i < numFields; i += 1 {
		field := v.Field(i)
		if !field.IsValid() || !field.CanSet() {
			continue
		}

		f := v.Type().Field(i)
		// convert name to snake_case
		name := toSnakeCase(f.Name)
		typ := f.Type
		kind := typ.Kind()
		optional := false
		if tag := f.Tag.Get("config"); tag != "" {
			for _, x := range strings.Split(tag, ",") {
				switch x {
				case "optional":
					optional = true
				default:
					name = x
				}
			}
		}

		val, ok := vals[name]
		if !ok && optional {
			continue
		} else if !ok && !optional {
			return fmt.Errorf(noField, path, name)
		}

		switch kind {
		case reflect.Int:
			intVal, err := strconv.ParseInt(val, 0, 64)
			if err != nil {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					err,
				)
			}

			if field.OverflowInt(intVal) {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					fmt.Errorf(overflow, intVal),
				)
			}

			field.SetInt(intVal)
		case reflect.Uint:
			intVal, err := strconv.ParseUint(val, 0, 64)
			if err != nil {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					err,
				)
			}

			if field.OverflowUint(intVal) {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					fmt.Errorf(overflow, intVal),
				)
			}

			field.SetUint(intVal)
		case reflect.String:
			field.SetString(val)
		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(val, 64)
			if err != nil {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					err,
				)
			}

			if field.OverflowFloat(floatVal) {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					fmt.Errorf(overflow, floatVal),
				)
			}
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(val)
			if err != nil {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					err,
				)
			}

			field.SetBool(boolVal)
		default:
			anyVal := field.Interface()
			p, ok := anyVal.(ValueParser)
			if !ok {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					fmt.Errorf(unsupported, typ.String()),
				)
			}

			if err := p.ParseConfigValue(val); err != nil {
				return fmt.Errorf(
					errorParsingConfig,
					path,
					err,
				)
			}
			field.Set(reflect.ValueOf(p).Elem())
		}
	}
	return nil
}

func toSnakeCase(x string) string {
	var b strings.Builder
	for i, c := range x {
		if i == 0 && unicode.IsUpper(c) {
			b.WriteRune(unicode.ToLower(c))
		} else if unicode.IsUpper(c) {
			b.WriteRune('_')
			b.WriteRune(unicode.ToLower(c))
		} else {
			b.WriteRune(c)
		}
	}

	return b.String()
}

func EnsureSet(vals ...string) {
	for _, v := range vals {
		if _, found := os.LookupEnv(v); !found {
			log.Fatalf("'%v' not set in .env or environment", v)
		}
	}
}
