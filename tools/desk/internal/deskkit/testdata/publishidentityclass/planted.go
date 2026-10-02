// Package planted is the class guard's positive control (issue #1967): a SECOND instance of
// the defect class the guard exists to catch. It is never compiled into anything (testdata is
// ignored by the go tool); TestPubClassGuardSeesPlant requires the guard to flag both shapes.
package planted

type PublishIdentityInput struct{ Dir, Base, RemoteTip, Role string }

func publishIdentityGate(dir, base, remoteTip string) error { return nil }

// pushAgain is an unreviewed caller offering a tip that is not a remote branch head at all.
func pushAgain(dir string) error {
	return publishIdentityGate(dir, "main", "refs/remotes/origin/HEAD")
}

// quietGate builds the input without stating RemoteTip — the zero value by omission.
func quietGate(dir string) PublishIdentityInput {
	return PublishIdentityInput{Dir: dir, Base: "main", Role: "worker"}
}
