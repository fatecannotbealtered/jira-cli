package cmd

// SilentErr sets the exit code and returns ErrSilent so cobra does not print again.
func SilentErr(code int) error {
	setExitCode(code)
	return ErrSilent
}
