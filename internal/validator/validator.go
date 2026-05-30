package validator

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"goboxd/internal/config"
	"goboxd/internal/types"
)

const (
	CodeBadJSON         = "bad_json"
	CodeUnknownLanguage = "unknown_language"
	CodeInvalidFilename = "invalid_filename"
	CodeSourceTooLarge  = "source_too_large"
	CodeStdinTooLarge   = "stdin_too_large"
	CodeTooManyTests    = "too_many_tests"
	CodeNoTests         = "no_tests"
	CodeFlagNotAllowed  = "flag_not_allowed"
)

type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Code + ": " + e.Message
}

func newErr(code, format string, args ...any) *ValidationError {
	return &ValidationError{Code: code, Message: fmt.Sprintf(format, args...)}
}

func Validate(req *types.RunRequest, reg *config.Registry, srv *config.ServerConfig) *ValidationError {
	lang, ok := reg.Lookup(req.Language)
	if !ok {
		return newErr(CodeUnknownLanguage, "unknown language %q", req.Language)
	}

	if !utf8.ValidString(req.Source) {
		return newErr(CodeSourceTooLarge, "source is not valid UTF-8")
	}

	if len(req.Source) > srv.MaxSourceBytes {
		return newErr(CodeSourceTooLarge, "source is %d bytes; max %d", len(req.Source), srv.MaxSourceBytes)
	}

	if err := validateFilename(req.SourceFilename, lang); err != nil {
		return err
	}

	if len(req.Tests) == 0 {
		return newErr(CodeNoTests, "at least one test is required")
	}

	if len(req.Tests) > srv.MaxTests {
		return newErr(CodeTooManyTests, "%d tests; max %d", len(req.Tests), srv.MaxTests)
	}

	for i, t := range req.Tests {
		if len(t.Stdin) > srv.MaxStdinBytes {
			return newErr(CodeStdinTooLarge, "test[%d] stdin is %d bytes; max %d", i, len(t.Stdin), srv.MaxStdinBytes)
		}
	}

	if req.Build != nil {
		if err := validateFlags(req.Build.Flags, lang.BuildFlagRules(), "build"); err != nil {
			return err
		}
	}

	if req.Run != nil {
		if err := validateFlags(req.Run.Flags, lang.RunFlagRules(), "run"); err != nil {
			return err
		}
	}

	return nil
}

func validateFilename(name string, lang *config.LanguageSpec) *ValidationError {
	switch lang.SourceFilenameStrategy {
	case config.FilenameFixed:
		if name == "" {
			return nil
		}
		if name != lang.SourceFilename {
			return newErr(CodeInvalidFilename, "language %q requires filename %q", lang.ID, lang.SourceFilename)
		}
		return nil
	case config.FilenameClientSupplied:
		if name == "" {
			return newErr(CodeInvalidFilename, "language %q requires source_filename", lang.ID)
		}

		if strings.ContainsAny(name, "/\\") {
			return newErr(CodeInvalidFilename, "filename must not contain path separators")
		}

		if strings.HasPrefix(name, ".") {
			return newErr(CodeInvalidFilename, "filename must not start with '.'")
		}

		if name == "." || name == ".." {
			return newErr(CodeInvalidFilename, "filename must not be '.' or '..'")
		}

		if len(name) > 64 {
			return newErr(CodeInvalidFilename, "filename longer than 64 bytes")
		}

		return nil
	}
	return nil
}

func validateFlags(flags []string, rules []config.FlagRule, phase string) *ValidationError {
	for _, f := range flags {
		if f == "" || strings.ContainsAny(f, " \t\n\r") {
			return newErr(CodeFlagNotAllowed, "%s flag %q contains whitespace or is empty", phase, f)
		}

		if !config.Allows(rules, f) {
			return newErr(CodeFlagNotAllowed, "%s flag %q not in allow-list", phase, f)
		}
	}
	return nil
}
