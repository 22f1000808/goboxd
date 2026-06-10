package validator

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"goboxd/internal/config"
	"goboxd/internal/types"
)

const (
	CodeBadJSON                = "bad_json"
	CodeUnknownLanguage        = "unknown_language"
	CodeInvalidFilename        = "invalid_filename"
	CodeSourceTooLarge         = "source_too_large"
	CodeStdinTooLarge          = "stdin_too_large"
	CodeExpectedStdoutTooLarge = "expected_stdout_too_large"
	CodeTooManyTests           = "too_many_tests"
	CodeNoTests                = "no_tests"
	CodeFlagNotAllowed         = "flag_not_allowed"
)

// denylistPrefixes are flag prefixes/tokens that are ALWAYS rejected
// regardless of the per-language allow-list. These are the exact patterns
// named in the spec §06 (security hole 3).
// Rejection produces HTTP 400 with code "flag_not_allowed".
var denylistPrefixes = []string{
	"-fplugin",
	"-x",
	"-B",
	"--specs",
	"-Wl,",
	"@", // response files
}

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
		if srv.MaxExpectedStdoutBytes > 0 && len(t.ExpectedStdout) > srv.MaxExpectedStdoutBytes {
			return newErr(CodeExpectedStdoutTooLarge, "test[%d] expected_stdout is %d bytes; max %d", i, len(t.ExpectedStdout), srv.MaxExpectedStdoutBytes)
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

// checkDenylist returns an error if the flag matches any unconditionally
// denied token. The denylist runs BEFORE the per-language allow-list.
func checkDenylist(flag string) *ValidationError {
	for _, prefix := range denylistPrefixes {
		if flag == prefix || strings.HasPrefix(flag, prefix) {
			return newErr(CodeFlagNotAllowed, "flag %q is not allowed (security policy)", flag)
		}
	}
	// Reject absolute-include guards: flags starting with -I/ or -I absolute paths
	// which could be used to include arbitrary host paths via compiler.
	if strings.HasPrefix(flag, "-I/") {
		return newErr(CodeFlagNotAllowed, "flag %q is not allowed (absolute include path)", flag)
	}
	return nil
}

func validateFlags(flags []string, rules []config.FlagRule, phase string) *ValidationError {
	for _, f := range flags {
		if f == "" || strings.ContainsAny(f, " \t\n\r") {
			return newErr(CodeFlagNotAllowed, "%s flag %q contains whitespace or is empty", phase, f)
		}

		// Denylist runs unconditionally before allowlist.
		if err := checkDenylist(f); err != nil {
			return err
		}

		if !config.Allows(rules, f) {
			return newErr(CodeFlagNotAllowed, "%s flag %q not in allow-list", phase, f)
		}
	}
	return nil
}
