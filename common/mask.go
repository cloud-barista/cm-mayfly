package common

// MaskSecret renders a credential for a log line: enough to tell one value from
// another, never enough to use.
//
// The -v flag exists so a user can see how a call was assembled, and answering
// "did my password reach the request at all, and is it the one I meant?" is a
// large part of that. Printing the value outright answers it at the cost of
// leaving the credential in a terminal scrollback, a CI job log or a redirected
// output file — places it outlives the debugging session by a long way. Printing
// nothing at all answers neither question.
//
// So the compromise is a short prefix: "(empty)" separates "not set" from "set",
// and the first two characters separate "the password I typed" from "some other
// password". Values short enough that a prefix would give away a meaningful
// share of them are replaced wholesale.
func MaskSecret(s string) string {
	if s == "" {
		return "(empty)"
	}
	r := []rune(s) // a prefix is taken in characters, so a multi-byte value is not cut mid-rune
	if len(r) <= 6 {
		return "***"
	}
	return string(r[:2]) + "***"
}
